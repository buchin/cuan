# Official CLI startup in experimental Docker isolation

The official Alpine/musl Antigravity CLI 1.4.3 installs with manifest SHA-512
verification and starts as the non-root cuan user inside Bubblewrap. A second
installer run validates the installed binary and reports already installed.
Both runs use the container's 128 MiB memory limit.

Version and help exit successfully; all six required flags are present.
Help is emitted on stderr, so the probe checks both output streams.
No account credentials were mounted and no inference was requested.
This proves startup and argument availability, not provider tool execution.

The existing boundary fixture still passes: read-only project/attachment,
writable selected synthetic profile, unrelated file and outer environment hidden,
private read-only procfs and self-executable helper. The application cannot create
namespaces; eleven present protected kernel paths deny access. Killing the
persistent parent leaves no sentinel descendants.

The exact worker and structured result are included. The experimental security
policies match ../docker-private-proc/: named AppArmor confinement and custom
seccomp, with OCI masked/readonly mounts replaced by explicit AppArmor denies.
This is not a supported installer policy or a completed alias/bypass review.

Cleanup verifies zero daemon/containerd processes, unloaded parent/child profiles,
unmounted private network namespace, removed image caches, and healthy unchanged
native production. See result.json for release and free disk.

The installer now hashes binaries in 1 MiB chunks. A separate targeted check
hashed 100,662,912 bytes under a 64 MiB address-space limit with the expected
SHA-512 result. Previously both installed and downloaded binaries were read
entirely into memory for those checks.

Still required: actual Cuan container task execution/recovery, policy review,
supported configuration and cross-host detection. Production was not deployed.
