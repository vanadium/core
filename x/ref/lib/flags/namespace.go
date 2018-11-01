package flags

import "fmt"

// NamespaceRootFlag represents a flag.Value for --v23.namespace.root.
type NamespaceRootFlag struct {
	isSet bool // is true when a flag has been explicitly set.
	// isDefault true when a flag has the default value and is needed in
	// addition to isSet to distinguish between using a default value
	// as opposed to one from an environment variable.
	isDefault bool
	Roots     []string
}

// String implements flag.Value.
func (nsr *NamespaceRootFlag) String() string {
	return fmt.Sprintf("%v", nsr.Roots)
}

// Set implements flag.Value
func (nsr *NamespaceRootFlag) Set(v string) error {
	nsr.isDefault = false
	if !nsr.isSet {
		// override the default value
		nsr.isSet = true
		nsr.Roots = []string{}
	}
	for _, t := range nsr.Roots {
		if v == t {
			return nil
		}
	}
	nsr.Roots = append(nsr.Roots, v)
	return nil
}
