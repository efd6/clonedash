// The clonedash program clones the entirety of an Elastic integration's
// dashboard assets under distinct identifiers as a deep copy either into
// a new namespace or the existing package's name space.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

func main() {
	root := flag.String("src", "", "specify the source package root")
	dst := flag.String("dst", "", "specify the destination package root (defaults to src package root)")
	dryRun := flag.Bool("dry-run", true, "only print the complete set of descriptions as a single JSON document with original file names")
	flag.Parse()
	if *root == "" {
		flag.Usage()
		os.Exit(2)
	}
	srcPkg := filepath.Base(*root)
	if *dst == "" {
		*dst = *root
	}
	dstPkg := filepath.Base(*dst)

	match, err := regexp.Compile(srcPkg + "-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
	if err != nil {
		log.Fatal(err)
	}
	kibana := filepath.Join(*root, "kibana")
	descriptions := map[string]map[string]map[string]interface{}{
		"dashboard":     make(map[string]map[string]interface{}),
		"search":        make(map[string]map[string]interface{}),
		"visualization": make(map[string]map[string]interface{}),
	}
	translation := make(map[string]string)
	err = filepath.WalkDir(kibana, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		path, err = filepath.Rel(kibana, path)
		if err != nil {
			return err
		}
		dir, file := filepath.Split(path)
		if dir == "" {
			return fmt.Errorf("unexpected number of path elements for %s", path)
		}
		dir = strings.TrimRight(dir, string(filepath.Separator))
		oldUUID := strings.TrimSuffix(file, ".json")
		for {
			newUUID := dstPkg + "-" + uuid.New().String()
			if _, exists := translation[newUUID]; exists {
				continue
			}
			translation[oldUUID] = newUUID
			break
		}
		var desc map[string]interface{}
		err = json.Unmarshal(b, &desc)
		if err != nil {
			return err
		}
		descriptions[dir][file] = desc

		return nil
	})
	if err != nil {
		log.Fatalf("error during walk: %v", err)
	}
	walk(descriptions, "", translation, match)
	if *dryRun {
		b, err := json.MarshalIndent(descriptions, "", "    ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", b)
		os.Exit(0)
	}
	for aspect, files := range descriptions {
		dstPath := filepath.Join(*dst, "kibana", aspect)
		fi, err := os.Stat(dstPath)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(dstPath, 0o755)
				if err != nil {
					log.Fatalf("failed to create destination directory: %v", err)
				}
			}
		} else if !fi.IsDir() {
			log.Fatalf("destination path is not a directory: %s", dstPath)
		}
		for file, desc := range files {
			file = translation[strings.TrimSuffix(file, ".json")] + ".json"
			b, err := json.MarshalIndent(desc, "", "    ")
			if err != nil {
				log.Fatalf("failed to marshal description: %v", err)
			}
			err = os.WriteFile(filepath.Join(dstPath, file), b, 0o644)
			if err != nil {
				log.Fatalf("failed to write file: %v", err)
			}
		}
	}
}

func walk(m interface{}, path string, translation map[string]string, match *regexp.Regexp) {
	switch m := m.(type) {
	case map[string]map[string]map[string]interface{}:
		for k, v := range m {
			walk(v, k, translation, match)
		}
	case map[string]map[string]interface{}:
		for k, v := range m {
			walk(v, path+"."+k, translation, match)
		}
	case map[string]interface{}:
		for k, v := range m {
			switch v := v.(type) {
			case string:
				if k != "id" {
					if match.MatchString(v) {
						m[k] = match.ReplaceAllStringFunc(v, func(s string) string {
							uuid, ok := translation[s]
							if !ok {
								return s
							}
							return uuid
						})
					}
					continue
				}
				uuid, ok := translation[v]
				if !ok {
					continue
				}
				m[k] = uuid
			default:
				walk(v, path+"."+k, translation, match)
			}
		}
	case []interface{}:
		for i, v := range m {
			walk(v, fmt.Sprintf(".%s[%d]", path, i), translation, match)
		}
	case nil, bool, float64, string:
	default:
		panic(fmt.Sprintf("unhanded type %s: %T", path, m))
	}
}
