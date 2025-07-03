# Lenovo Fan Control Go

`lenovo_fan_control_go` is a Lenovo fan control tool written in Go. It offers both console and graphical user interface (GUI) operation modes, allowing users to conveniently control the fan modes of Lenovo devices.

## Features
- **Mode Switching**: Supports switching the fan mode between Normal Mode (NORMAL) and High-Speed Mode (FAST).
- **Sustained High-Speed Mode**: You can set the duration for the high-speed mode. The fan will automatically revert to normal mode after the specified time.
- **Status Query**: Enables users to retrieve the current operating status of the fan.
- **Signal Handling**: The console version can capture the `Ctrl+C` signal, restore the fan to normal mode, and then exit.

## Dependencies
This project depends on Go 1.24.1 and the following third - party libraries:
- `fyne.io/fyne/v2 v2.6.1`: Used for building the graphical user interface.

For the complete list of dependencies, please refer to the `go.mod` file.