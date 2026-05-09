package adminui

// NavClass returns sidebar link classes for the active section.
func NavClass(active, section string) string {
	if active == section {
		return "nav-item nav-item-active"
	}
	return "nav-item"
}
