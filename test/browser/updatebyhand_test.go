//go:build browser

package browser

import (
	"strings"
	"testing"
)

// A container without the updater is told how to update by hand, and offered
// no button.
//
// The updater holds the Docker socket, which is root on the host, so an
// installation may leave it out on purpose and update its image by hand. It
// still has to learn that there is something to update to: the banner across
// every screen names the new version, and the version card says which command
// takes it, in every language, with no button beside it - a binary swapped
// inside the container would be undone by the next recreate. The state is the
// one the server answers in such a container; the harness runs the application
// on a host, so the page is handed it.
func TestAContainerWithoutTheUpdaterIsToldHowToUpdateByHand(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#update-card")

	const command = "docker compose pull && docker compose up -d"

	for _, language := range []string{"en", "de"} {
		if language != "en" {
			p.chooseLanguage(language)
		}

		var card struct {
			Line   string `json:"line"`
			Hint   string `json:"hint"`
			Button bool   `json:"button"`
			Banner string `json:"banner"`
		}

		// After the language, because choosing one redraws every card from the
		// server, which would put this installation's own state over the one
		// handed in.
		p.evalJSON(`JSON.stringify((() => {
			const state = {
				running: 'v0.4.17', latest: 'v0.4.18', newer: true, available: false,
				installable: false, why: 'inContainer', restartable: true, enabled: true,
				comparable: true, url: 'https://example.com/release',
			};

			document.querySelector('#update-card').hidden = false;
			redrawable('update', () => renderUpdate(state));
			showReleaseState(state);

			const banner = document.querySelector('#release-banner');

			return {
				line: document.querySelector('#update-state').textContent,
				hint: document.querySelector('#update-hint').textContent,
				button: document.querySelector('#update-now').offsetParent !== null,
				banner: banner.hidden ? '' : banner.textContent,
			};
		})())`, &card)

		if !strings.Contains(card.Line, "v0.4.18") {
			t.Errorf("%s: the card does not name the newer version: %q", language, card.Line)
		}

		for _, name := range []string{command, "deploy/compose.update.yaml"} {
			if !strings.Contains(card.Hint, name) {
				t.Errorf("%s: the card does not say %q:\n\t%s", language, name, card.Hint)
			}
		}

		if card.Button {
			t.Errorf("%s: the card offers to install in a container, where the next recreate undoes it", language)
		}

		if !strings.Contains(card.Banner, "v0.4.18") {
			t.Errorf("%s: no banner names the newer version: %q", language, card.Banner)
		}
	}
}
