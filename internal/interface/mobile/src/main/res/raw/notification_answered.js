// Tells the page that the phone has answered the Reminders card's question about
// notifications, so that the card is drawn again with what the phone now allows;
// see Notifications in Web.kt and appNotifications in app.js.
window.dispatchEvent(new Event('gts-notifications'));
