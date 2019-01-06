package sitedefaults

// Defaults is a placeholder that is intended to be overridden by each
// site-specific installation to provide site-specific defaults for all
// compiled-in defaults for command line flags. v.io/x/ref/lib/flags imports
// this package and uses the defaults provided here in preference to its own.
var Defaults = map[string]interface{}{}
