package skills

import "strings"

func RestoreCapabilityAsSkill(capName, description string) *Skill {
	return &Skill{
		ID:          strings.ReplaceAll(capName, "_", "-"),
		Name:        strings.Title(strings.ReplaceAll(capName, "_", " ")),
		Description: description,
		Body:        "# " + strings.Title(strings.ReplaceAll(capName, "_", " ")) + "\n\n" + description,
	}
}
