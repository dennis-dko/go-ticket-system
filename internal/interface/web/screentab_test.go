package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every card on a screen with tabs names a tab that screen has, and every tab has
// a panel and something on it.
//
// The cards are moved into their tabs' panels by the script, by the name each
// card carries: one that names no tab, or one the screen does not have, is moved
// nowhere and stays where the markup put it - outside every panel, on screen
// whichever tab is chosen, or hidden with none of them. Nothing would say so
// until somebody looked. A card that names a tab off such a screen is a mistake
// the other way round, so it is reported too.
func TestEveryCardOnATabbedScreenNamesATabItHas(t *testing.T) {
	markup := readAsset(t, "index.html")

	cardTab := regexp.MustCompile(`\sdata-tab="([^"]+)"`)
	tabs := regexp.MustCompile(`<button [^>]*role="tab"[^>]*>`)
	panels := regexp.MustCompile(`<div [^>]*role="tabpanel"[^>]*>`)
	attribute := func(name string) *regexp.Regexp { return regexp.MustCompile(`\s` + name + `="([^"]+)"`) }

	screens := 0

	for _, section := range regexp.MustCompile(`(?s)<section id="view-[a-z]+".*?</section>`).FindAllString(markup, -1) {
		id := elementID.FindStringSubmatch(section)[1]

		if !strings.Contains(section, `class="screen-tabs"`) {
			if found := cardTab.FindString(section); found != "" {
				t.Errorf("%s has no tabs, and a card on it names one:%s", id, found)
			}

			continue
		}

		screens++

		var offered, panelled []string

		for _, tab := range tabs.FindAllString(section, -1) {
			key := attribute("data-card-tab").FindStringSubmatch(tab)
			controls := attribute("aria-controls").FindStringSubmatch(tab)

			if key == nil || controls == nil {
				t.Errorf("%s has a tab that names no key or no panel: %s", id, tab)

				continue
			}

			offered = append(offered, key[1])

			if !strings.Contains(section, `id="`+controls[1]+`"`) {
				t.Errorf("%s's tab %q controls %q, which is not on the screen", id, key[1], controls[1])
			}
		}

		for _, panel := range panels.FindAllString(section, -1) {
			if key := attribute("data-card-panel").FindStringSubmatch(panel); key != nil {
				panelled = append(panelled, key[1])
			}
		}

		if !slices.Equal(offered, panelled) {
			t.Errorf("%s offers the tabs %q and has panels for %q", id, offered, panelled)
		}

		used := map[string]int{}

		for _, card := range cardOpening.FindAllString(section, -1) {
			name := cardTab.FindStringSubmatch(card)
			if name == nil {
				t.Errorf("a card on %s names no tab, so it is on none of them: %s", id, card)

				continue
			}

			if !slices.Contains(offered, name[1]) {
				t.Errorf("a card on %s names the tab %q, which the screen does not have: %s", id, name[1], card)
			}

			used[name[1]]++
		}

		for _, key := range offered {
			if used[key] == 0 {
				t.Errorf("%s's tab %q has no card on it", id, key)
			}
		}
	}

	// The five the maintainer asked for: My account and Settings on 2026-11-18,
	// time entries, the calendar and the report on 2026-11-19. Counted, so a
	// sixth is a decision somebody writes down here rather than one that happens.
	if screens != 5 {
		t.Errorf("%d screens have tabs, want My account, Settings, time entries, the calendar and the report",
			screens)
	}
}
