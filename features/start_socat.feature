Feature: Start Socat
  When slavesim is configured with an RTU transport, it automatically
  starts socat to create a virtual serial port pair. When slavesim
  exits, socat is stopped automatically.

  Scenario: Socat starts when RTU transport is configured
    Given slavesim is configured with RTU transport at "/tmp/ttyV0"
    When slavesim starts
    Then socat is running
    And the virtual TTY "/tmp/ttyV0" exists

  Scenario: Socat stops when slavesim exits
    Given slavesim is running with RTU transport at "/tmp/ttyV0"
    When slavesim exits
    Then socat is no longer running
    And the virtual TTY "/tmp/ttyV0" no longer exists

  Scenario: No socat started for TCP-only configuration
    Given slavesim is configured with TCP transport only
    When slavesim starts
    Then socat is not running

  Scenario: Error when socat is not installed
    Given socat is not installed on the system
    And slavesim is configured with RTU transport
    When slavesim starts
    Then an error message "socat not found: install socat first" is shown
    And slavesim exits with a non-zero status

  Scenario: Error when virtual TTY is not created in time
    Given slavesim is configured with RTU transport at "/tmp/ttyV0"
    When socat starts but the virtual TTY is not created within 100ms
    Then an error message "/tmp/ttyV0 doesn't exist" is shown
    And slavesim exits with a non-zero status
