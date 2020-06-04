// Code generated by github.com/go-enry/go-enry/v2/internal/code-generator DO NOT EDIT.
// Extracted from github/linguist commit: 40992ba7f86889f80dfed3ba95e11e1082200bad

package data

import "github.com/go-enry/go-enry/v2/regex"

var DocumentationMatchers = []regex.EnryRegexp{
	regex.MustCompile(`^[Dd]ocs?/`),
	regex.MustCompile(`(^|/)[Dd]ocumentation/`),
	regex.MustCompile(`(^|/)[Gg]roovydoc/`),
	regex.MustCompile(`(^|/)[Jj]avadoc/`),
	regex.MustCompile(`^[Mm]an/`),
	regex.MustCompile(`^[Ee]xamples/`),
	regex.MustCompile(`^[Dd]emos?/`),
	regex.MustCompile(`(^|/)inst/doc/`),
	regex.MustCompile(`(^|/)CHANGE(S|LOG)?(\.|$)`),
	regex.MustCompile(`(^|/)CONTRIBUTING(\.|$)`),
	regex.MustCompile(`(^|/)COPYING(\.|$)`),
	regex.MustCompile(`(^|/)INSTALL(\.|$)`),
	regex.MustCompile(`(^|/)LICEN[CS]E(\.|$)`),
	regex.MustCompile(`(^|/)[Ll]icen[cs]e(\.|$)`),
	regex.MustCompile(`(^|/)README(\.|$)`),
	regex.MustCompile(`(^|/)[Rr]eadme(\.|$)`),
	regex.MustCompile(`^[Ss]amples?/`),
}
