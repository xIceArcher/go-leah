package instagram

import "regexp"

var HashtagRegex = regexp.MustCompile(`(?:[#|＃])[\p{L}\p{M}\p{N}][\p{L}\p{M}\p{N}_]+`)
var MentionRegex = regexp.MustCompile(`(?:[@|＠])[A-Za-z0-9\._]+`)
