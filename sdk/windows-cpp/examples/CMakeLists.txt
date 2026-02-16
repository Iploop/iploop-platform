cmake_minimum_required(VERSION 3.16)

# Examples
add_executable(basic_example basic_example.cpp)
target_link_libraries(basic_example IPLoopSDK)

add_executable(advanced_example advanced_example.cpp)  
target_link_libraries(advanced_example IPLoopSDK)

# v2.0: Binary protocol example
add_executable(v2_binary_example v2_binary_example.cpp)
target_link_libraries(v2_binary_example IPLoopSDK)

add_executable(c_api_example c_api_example.c)
target_link_libraries(c_api_example IPLoopSDK)

# Console app for testing
add_executable(console_test console_test.cpp)
target_link_libraries(console_test IPLoopSDK)

# Install examples
install(TARGETS basic_example advanced_example v2_binary_example c_api_example console_test
        DESTINATION bin/examples)