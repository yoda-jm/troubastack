/*
 * DESK01 D3 — the universal TroubaStage.app's executable. macOS runs the slice that matches the Mac (or Rosetta's
 * x86_64 one when asked); that slice replaces itself with the jpackage launcher built for the same architecture,
 * TroubaStage-arm64 or TroubaStage-x86_64 next to it, which starts the Java runtime built for that architecture
 * (its own cfg's app.runtime). Nothing else: the arguments and the environment pass through unchanged.
 */
#include <libgen.h>
#include <limits.h>
#include <mach-o/dyld.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#if defined(__arm64__)
#define ARCH "arm64"
#elif defined(__x86_64__)
#define ARCH "x86_64"
#else
#error "TroubaStage is built for arm64 and x86_64 only"
#endif

int main(int argc, char *argv[]) {
    char self[PATH_MAX], real[PATH_MAX], target[PATH_MAX];
    uint32_t size = sizeof self;
    if (_NSGetExecutablePath(self, &size) != 0 || realpath(self, real) == NULL) {
        fprintf(stderr, "TroubaStage: cannot find its own path\n");
        return 70;
    }
    snprintf(target, sizeof target, "%s/TroubaStage-" ARCH, dirname(real));
    argv[0] = target;
    execv(target, argv);
    perror(target);
    return 71;
}
