// Tells the page that the phone has answered the Reminders card's question about
// notifications, so that the card is drawn again with what the phone now allows;
// see Notifications in Web.kt and appNotifications in app.js. Under the name a
// page from before the rename to TimePulse listens for, too.
window.dispatchEvent(new Event('timepulse-notifications'));
window.dispatchEvent(new Event('gtr-notifications'));
