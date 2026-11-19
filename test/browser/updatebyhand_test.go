//go:build browser

package browser

import (
	"strings"
	"testing"
)

// updateCard is what the version card and the banner say for one state.
type updateCard struct {
	Line   string `json:"line"`
	Hint   string `json:"hint"`
	ByHand string `json:"byHand"`
	Button bool   `json:"button"`
	Banner string `json:"banner"`
}

// containerWithoutUpdater is what the server answers in a container with a
// newer release and nothing beside it that can replace the image.
const containerWithoutUpdater = `{
	running: 'v0.4.17', latest: 'v0.4.18', newer: true, available: false,
	installable: false, why: 'inContainer', restartable: true, enabled: true,
	comparable: true, url: 'https://example.com/release',
}`

// hostWithANewerRelease is what it answers for a binary on a host, which can
// install the release itself.
const hostWithANewerRelease = `{
	running: 'v0.4.17', latest: 'v0.4.18', newer: true, available: true,
	installable: true, restartable: true, enabled: true,
	comparable: true, url: 'https://example.com/release',
}`

// drawUpdate hands the page an update state and reads back what the card and
// the banner say about it.
func (p *page) drawUpdate(state string) updateCard {
	p.t.Helper()

	var card updateCard

	p.evalJSON(`JSON.stringify((() => {
		const state = `+state+`;

		document.querySelector('#update-card').hidden = false;
		redrawable('update', () => renderUpdate(state));
		showReleaseState(state);

		const shown = (selector) => {
			const node = document.querySelector(selector);
			return node && node.offsetParent !== null ? node.textContent : '';
		};

		return {
			line: shown('#update-state'),
			hint: shown('#update-hint'),
			byHand: shown('#update-by-hand'),
			button: document.querySelector('#update-now').offsetParent !== null,
			banner: shown('#release-banner'),
		};
	})())`, &card)

	return card
}

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
//
// The command is the one for Compose with its overlays listed in COMPOSE_FILE,
// and the guide itself describes three deployments it does not take: overlays
// given with -f, which it would leave out, a version pinned in
// TIMEPULSE_VERSION, which it would pull again, and a container started with
// docker run. The card names all three, and only where it names the command.
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

		// After the language, because choosing one redraws every card from the
		// server, which would put this installation's own state over the one
		// handed in.
		card := p.drawUpdate(containerWithoutUpdater)

		if !strings.Contains(card.Line, "v0.4.18") {
			t.Errorf("%s: the card does not name the newer version: %q", language, card.Line)
		}

		for _, name := range []string{command, "deploy/compose.update.yaml"} {
			if !strings.Contains(card.Hint, name) {
				t.Errorf("%s: the card does not say %q:\n\t%s", language, name, card.Hint)
			}
		}

		for _, name := range []string{"-f", "COMPOSE_FILE", "TIMEPULSE_VERSION", "docker run", "docker pull"} {
			if !strings.Contains(card.ByHand, name) {
				t.Errorf("%s: the card does not say where the command alone is not enough - %q is missing:\n\t%s",
					language, name, card.ByHand)
			}
		}

		if card.Button {
			t.Errorf("%s: the card offers to install in a container, where the next recreate undoes it", language)
		}

		if !strings.Contains(card.Banner, "v0.4.18") {
			t.Errorf("%s: no banner names the newer version: %q", language, card.Banner)
		}
	}

	// A host installs the release itself, so it has the button and nothing to do
	// by hand.
	host := p.drawUpdate(hostWithANewerRelease)

	if !host.Button {
		t.Error("a host with a newer release is offered no button to install it")
	}

	if host.ByHand != "" {
		t.Errorf("a host is told how to update a container by hand:\n\t%s", host.ByHand)
	}
}
