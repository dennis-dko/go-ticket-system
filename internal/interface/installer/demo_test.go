package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// The question whether the new database is to hold demo data travels with the
// connection it is asked of, in the language the page was read in, and nothing
// asks for demo data on its own.
//
// The installer only carries the question: the application answers it once it
// has migrated the database, because only then is there anything to fill. So
// what has to hold here is that the save hands it over intact, and that a save
// which did not tick the box - every installation that will be worked in - is
// handed over as not asking.
func TestASaveHandsOverWhetherDemoDataWasAskedFor(t *testing.T) {
	for _, c := range []struct {
		name     string
		extra    string
		demo     bool
		language string
	}{
		{name: "asked, in German", extra: `,"demo":true,"language":"de"`, demo: true, language: "de"},
		{name: "not asked", extra: ``, demo: false, language: ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()

			s := &server{
				cfg:  Config{Token: "the-token", DatasourceFile: filepath.Join(dir, "datasource.json"), Logf: t.Logf},
				done: make(chan handover, 1),
			}

			body := fmt.Sprintf(`{"dialect":"sqlite","name":%q%s}`,
				filepath.ToSlash(filepath.Join(dir, "timepulse")), c.extra)
			req := httptest.NewRequest(http.MethodPost, "/install/save", strings.NewReader(body))
			req.Header.Set("X-Setup-Token", "the-token")

			rec := httptest.NewRecorder()
			s.save(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("the answer was refused with %d: %s", rec.Code, rec.Body)
			}

			handed := <-s.done

			if handed.demo != c.demo || handed.language != c.language {
				t.Errorf("handed over demo %v in %q, want %v in %q",
					handed.demo, handed.language, c.demo, c.language)
			}
		})
	}
}
