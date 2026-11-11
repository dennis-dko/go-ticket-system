//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// A step the browser refuses is said in the screen's words, as every other
// refusal is, and the steps that are left are the server's: whole hours for the
// stopwatch's reminder, while a daily target of 7.75 hours - a 38.75-hour week,
// which the form refused in steps of a half - is taken as the server takes it.
func TestAStepIsTheServersAndSaidInTheScreensWords(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.showView("settings")
	p.waitShown("#form-reminders")

	p.run("ask for the stopwatch after two and a half hours",
		chromedp.SetValue(`#form-reminders input[name="timerAlertHours"]`, "2.5", chromedp.ByQuery),
		p.click(`#form-reminders button[type="submit"]`))
	p.waitEvaluates("what the field says",
		`document.querySelector('#form-reminders input[name="timerAlertHours"]').validationMessage`,
		"Please enter a whole number.")

	var taken bool

	p.run("type a daily target of 7.75 hours", chromedp.Evaluate(`(() => {
		const field = document.querySelector('#form-working-times input[name="dailyTargetHours"]');
		field.value = '7.75';
		return field.validity.valid;
	})()`, &taken))

	if !taken {
		t.Error("the form refuses a daily target of 7.75 hours, which the server takes")
	}
}
