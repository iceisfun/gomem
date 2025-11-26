#include <iostream>
#include <vector>
#include <string>
#include <cstring>
#include <thread>
#include <chrono>
#include <unistd.h>

// Simple struct to be pointed to
struct FlagData {
    int id;
    char name[32];
    float value;
};

// Nested struct with data
struct InnerData {
    int someInteger;
    FlagData* flagPtr;
    char description[64];
};

// Main struct to be searched for
struct GameState {
    char seed[4]; // "SEED"
    uint64_t uniqueId;
    InnerData inner;
    FlagData* otherFlagPtr;
};

#include <sys/prctl.h>

int main() {
    // Allow ptrace from any process (for testing purposes)
    prctl(PR_SET_PTRACER, PR_SET_PTRACER_ANY, 0, 0, 0);

    std::cout << "Test Program Started. PID: " << getpid() << std::endl;

    // Allocate data on heap to ensure it's not just on stack
    FlagData* flag1 = new FlagData{1, "CaptureTheFlag", 3.14f};
    FlagData* flag2 = new FlagData{2, "KingOfTheHill", 9.99f};

    GameState* state = new GameState();
    
    // Initialize with "SEED" marker
    std::memcpy(state->seed, "SEED", 4);
    state->uniqueId = 0xDEADBEEFCAFEBABE;
    
    // Setup inner data
    state->inner.someInteger = 42;
    state->inner.flagPtr = flag1;
    std::strcpy(state->inner.description, "This is a test description");

    // Setup other pointer
    state->otherFlagPtr = flag2;

    std::cout << "GameState address: " << state << std::endl;
    std::cout << "Flag1 address: " << flag1 << std::endl;
    std::cout << "Flag2 address: " << flag2 << std::endl;
    std::cout << "Waiting for scanner... (Press Ctrl+C to stop)" << std::endl;

    // Keep the program running and update a value occasionally
    int counter = 0;
    while (true) {
        state->inner.someInteger = counter++;
        std::this_thread::sleep_for(std::chrono::seconds(1));
        if (counter % 10 == 0) {
            std::cout << "Still running... Counter: " << counter << std::endl;
        }
    }

    delete flag1;
    delete flag2;
    delete state;
    return 0;
}
