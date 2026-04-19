Feature: Write Register
  The user can interactively write a value to a slave register using
  the command "w <unitID> <addr> <value>" or its long form "write".
  The value type is inferred automatically: integers map to uint16,
  decimal numbers to float32, and true/false to bool.

  Scenario: Write a uint16 value to a register
    Given slave 1 is connected
    When the user enters "w 1 0x73ee 1"
    Then register 0x73ee on slave 1 contains uint16 value 1

  Scenario: Write a float32 value to two consecutive registers
    Given slave 1 is connected
    When the user enters "w 1 0x73ee 3.14"
    Then register 0x73ee on slave 1 contains the high word of float32 3.14
    And register 0x73ef on slave 1 contains the low word of float32 3.14

  Scenario: Write bool true to a register
    Given slave 1 is connected
    When the user enters "w 1 0x73ee true"
    Then register 0x73ee on slave 1 contains uint16 value 1

  Scenario: Write bool false to a register
    Given slave 1 is connected
    When the user enters "w 1 0x73ee false"
    Then register 0x73ee on slave 1 contains uint16 value 0

  Scenario: Long form "write" is accepted
    Given slave 1 is connected
    When the user enters "write 1 0x73ee 42"
    Then register 0x73ee on slave 1 contains uint16 value 42

  Scenario: Slave does not exist
    Given no slave with id 1 is connected
    When the user enters "w 1 0x73ee 1"
    Then an error message "slave 1 not found" is shown

  Scenario: Invalid register address
    Given slave 1 is connected
    When the user enters "w 1 0xZZZZ 1"
    Then an error message "invalid address: 0xZZZZ" is shown

  Scenario: Invalid value
    Given slave 1 is connected
    When the user enters "w 1 0x73ee maybe"
    Then an error message "invalid value: maybe" is shown

  Scenario: Missing arguments
    When the user enters "w 1 0x73ee"
    Then an error message "usage: w <unitID> <addr> <value>" is shown
