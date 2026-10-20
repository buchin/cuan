# Cuan sandbox code in an Alpine container

Four actual Cuan Go tests run without skips inside the confined experimental
Alpine container as non-root: filesystem boundary, broad workspace rejection,
missing sandbox fails closed, and official CLI help through sandboxInboxCommand.
The test binary is compiled with CGO disabled from clean f0aa447 plus the included
test-only shell fixture correction. The boundary fixture uses dash if available:
BusyBox applet dispatch cannot use argv[0] /proc/self/exe as a shell helper.

The official musl AGY1.4.3 starts through Cuan's actual argument builder.
No accounts or provider inference are used. This closes the gap between the
standalone boundary fixture and Cuan's sandbox builder, not full image task
execution/recovery or policy bypass review. The existing parent-loss and kernel
path fixtures pass too. Exact worker and cleanup/health evidence are included.
All experimental profiles, daemon processes, image caches and private mounts
were cleaned; native production remains unchanged and healthy.
