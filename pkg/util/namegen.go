package util

import (
	"math/rand/v2"
	"strings"
)

var (
	nameAdjectives = [...]string{
		"amber", "ancient", "azure", "blue", "bold", "brave", "bright", "bronze",
		"calm", "candid", "clear", "clever", "cobalt", "cool", "copper", "coral",
		"crimson", "crisp", "curious", "daring", "dawn", "deep", "distant", "dusky",
		"eager", "early", "elder", "emerald", "fair", "faithful", "fleet", "fond",
		"gentle", "gilded", "glad", "golden", "grand", "grateful", "green", "hardy",
		"honest", "humble", "indigo", "ivory", "jade", "jolly", "keen", "kind",
		"lively", "loyal", "lucid", "lunar", "merry", "mighty", "mild", "misty",
		"modest", "noble", "northern", "olive", "opal", "patient", "pearl", "polar",
		"proud", "quiet", "rapid", "royal", "ruby", "rustic", "sage", "sapphire",
		"scarlet", "serene", "sharp", "silver", "sincere", "slate", "solar", "sound",
		"southern", "spry", "stark", "steady", "stellar", "sterling", "still", "stout",
		"sunny", "sweet", "tawny", "teal", "tidy", "topaz", "true", "twilight",
		"valiant", "velvet", "violet", "vivid", "warm", "westerly", "willing", "wise",
	}

	nameAdverbs = [...]string{
		"boldly", "brightly", "briskly", "calmly", "candidly", "clearly", "cleverly", "closely",
		"deeply", "dearly", "eagerly", "early", "evenly", "fairly", "firmly", "fondly",
		"freely", "gently", "gladly", "gracefully", "happily", "keenly", "kindly", "lightly",
		"merrily", "mildly", "neatly", "nobly", "openly", "plainly", "politely", "proudly",
		"quietly", "rapidly", "readily", "richly", "sharply", "simply", "sincerely", "smoothly",
		"softly", "steadily", "sternly", "stoutly", "sweetly", "swiftly", "tidily", "truly",
		"warmly", "wisely",
	}

	nameNouns = [...]string{
		"almanac", "anchor", "antenna", "arbor", "arc", "arcade", "archive", "arrow",
		"atlas", "aurora", "aviary", "basin", "beacon", "beam", "bell", "bellows",
		"belt", "borealis", "boulder", "bridge", "brook", "cairn", "camp", "canyon",
		"cape", "caravan", "cascade", "cavern", "cedar", "chapel", "chart", "chime",
		"cinder", "cistern", "citadel", "cliff", "clock", "cloud", "clover", "comet",
		"compass", "cove", "crater", "creek", "crest", "crossing", "crown", "delta",
		"dial", "dock", "dune", "dusk", "eclipse", "ember", "estuary", "fathom",
		"fern", "ferry", "fjord", "flint", "forge", "fountain", "gable", "garden",
		"gate", "geyser", "glacier", "glade", "granite", "grotto", "grove", "gully",
		"halo", "hamlet", "harbor", "haven", "hearth", "heath", "hedge", "highland",
		"hollow", "horizon", "isle", "jetty", "junction", "keystone", "kiln", "lagoon",
		"lantern", "ledger", "lens", "lighthouse", "lodge", "lookout", "loom", "meadow",
		"mesa", "meridian", "mesh", "mill", "mirror", "moor", "mosaic", "nebula",
		"nook", "oasis", "obelisk", "orbit", "orchard", "outpost", "oxbow", "pagoda",
		"pantry", "parapet", "pass", "pasture", "path", "pavilion", "peak", "pebble",
		"pendulum", "pier", "pillar", "pinnacle", "plateau", "plaza", "pond", "portal",
		"prairie", "prism", "quarry", "quartz", "quay", "quill", "radar", "rampart",
		"ranger", "rapids", "ravine", "reef", "ridge", "rill", "rivulet", "sanctum",
		"sandbar", "satchel", "scarp", "sextant", "shoal", "shore", "signal", "silo",
		"solstice", "sound", "spire", "spring", "sprout", "spyglass", "steeple", "stone",
		"strait", "summit", "sundial", "tarn", "telescope", "terrace", "thicket", "tide",
		"timber", "torch", "tower", "trail", "trellis", "tundra", "valley", "vault",
		"vista", "voyage", "wagon", "waypoint", "wharf", "willow", "windmill", "zenith",
	}
)

type NameGenerator struct {
	rnd *rand.Rand
}

// NewNameGenerator uses the global random source when rnd is nil.
func NewNameGenerator(rnd *rand.Rand) *NameGenerator {
	return &NameGenerator{rnd: rnd}
}

func (g *NameGenerator) intn(n int) int {
	if g.rnd == nil {
		return rand.IntN(n)
	}
	return g.rnd.IntN(n)
}

func (g *NameGenerator) Adverb() string {
	return nameAdverbs[g.intn(len(nameAdverbs))]
}

func (g *NameGenerator) Adjective() string {
	return nameAdjectives[g.intn(len(nameAdjectives))]
}

func (g *NameGenerator) Noun() string {
	return nameNouns[g.intn(len(nameNouns))]
}

// Generate builds a name of the given word count: one word is a noun, two is an
// adjective and a noun, and any more prefixes adverbs.
func (g *NameGenerator) Generate(words int, separator string) string {
	switch {
	case words <= 1:
		return g.Noun()
	case words == 2:
		return g.Adjective() + separator + g.Noun()
	}
	parts := make([]string, 0, words)
	for range words - 2 {
		parts = append(parts, g.Adverb())
	}
	parts = append(parts, g.Adjective(), g.Noun())
	return strings.Join(parts, separator)
}

var defaultNameGenerator = NewNameGenerator(nil)

func GenerateName(words int, separator string) string {
	return defaultNameGenerator.Generate(words, separator)
}
