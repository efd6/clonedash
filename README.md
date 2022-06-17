# Clonedash

The `clonedash` program clones the entirety of an Elastic integration's
dashboard assets under distinct identifiers as a deep copy either into
a new namespace or the existing package's name space.

Note: UUIDs are guaranteed to not collide if cloning within a package,
collisions when cloning into another package namespace with existing assets
follow UUIDv4 behaviour ... still safe.