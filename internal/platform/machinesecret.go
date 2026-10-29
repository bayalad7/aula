package platform

import "fmt"

// RestrictMachineSecret protects a secret shared by the users of a machine,
// such as the site key on a teacher computer: administrators and the system
// get full control, local users read only (so every teacher who logs in can
// use it), nobody else anything. On Windows the DACL is replaced and
// protected; elsewhere the mode becomes 0640 (files) or 0750 (directories).
func RestrictMachineSecret(path string) error {
	if err := restrictMachineSecret(path); err != nil {
		return fmt.Errorf("restrict %s: %w", path, err)
	}
	return nil
}
