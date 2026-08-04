package sop

type ReviewPolicy struct {
	RequireDifferentBreed bool
	RequireDifferentCLI  bool
	PreferredRoles       []string
	ExcludeUnavailable   bool
}

type ReviewResult struct {
	Status   string
	Comments string
	Reviewer string
}

func SelectReviewer(authorBreed string, candidates []string, policy ReviewPolicy) string {
	for _, c := range candidates {
		if policy.RequireDifferentBreed && c == authorBreed {
			continue
		}
		return c
	}
	return ""
}
