#include <windows.h>
#include <stdint.h>
#include <stdio.h>

/* Diagnose the address-space cost of the x86 Go DLL independently of a game.
 * This process is deliberately not LARGEADDRESSAWARE. No memory map or game
 * configuration is changed outside this short-lived test process. */
static void snapshot(const char *stage) {
    uintptr_t address = 0;
    unsigned long long available = 0, reserved = 0, committed = 0, largest = 0;
    MEMORY_BASIC_INFORMATION info;
    while (address < 0x80000000u && VirtualQuery((void *)address, &info, sizeof(info))) {
        unsigned long long size = info.RegionSize;
        if (size > 0x80000000ULL - address) size = 0x80000000ULL - address;
        if (info.State == MEM_FREE) {
            available += size;
            if (size > largest) largest = size;
        } else if (info.State == MEM_RESERVE) reserved += size;
        else if (info.State == MEM_COMMIT) committed += size;
        if (!size) break;
        address += (uintptr_t)size;
    }
    printf("%s free_mib=%.2f reserved_mib=%.2f committed_mib=%.2f largest_free_mib=%.2f ACP=%u\n",
        stage, available/1048576.0, reserved/1048576.0, committed/1048576.0, largest/1048576.0, GetACP());
}

int wmain(int argc, wchar_t **argv) {
    if (argc != 2 || sizeof(void *) != 4) return 2;
    snapshot("baseline");
    HMODULE dll = LoadLibraryW(argv[1]);
    if (!dll) { printf("LoadLibrary error=%lu\n", GetLastError()); return 1; }
    /* Go's DLL initialization starts a runtime thread. Sample after it settles. */
    Sleep(1000);
    snapshot("loaded_without_hooks");
    typedef unsigned int (__cdecl *install_fn)(void *);
    install_fn install = (install_fn)GetProcAddress(dll, "InstallLocale");
    if (!install) return 1;
    unsigned int count = install(NULL);
    printf("installed_hooks=%u\n", count);
    snapshot("with_hooks");
    /* Never unload a c-shared Go DLL with live runtime threads. */
    return count ? 0 : 1;
}
