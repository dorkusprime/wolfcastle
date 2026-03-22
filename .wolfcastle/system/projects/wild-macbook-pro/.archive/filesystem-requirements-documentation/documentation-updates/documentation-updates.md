# Documentation Updates

Add filesystem requirements to README.md and SECURITY.md documenting that .wolfcastle/ must reside on a local filesystem due to flock(2) advisory locking limitations on NFS/network filesystems.
