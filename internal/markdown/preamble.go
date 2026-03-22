package markdown

// IsPreamble returns true if the section is a preamble (content before the first ## heading).
func (s Section) IsPreamble() bool {
	return s.Heading == "" && s.Depth == 0
}

// HasPreamble returns true if the first section in the list is a preamble.
func HasPreamble(sections []Section) bool {
	return len(sections) > 0 && sections[0].IsPreamble()
}

// SplitPreamble separates sections into preamble (if present) and body sections.
func SplitPreamble(sections []Section) (preamble *Section, body []Section) {
	if len(sections) == 0 {
		return nil, nil
	}
	if sections[0].IsPreamble() {
		return &sections[0], sections[1:]
	}
	return nil, sections
}
