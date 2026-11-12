/*
 * What lets the installed application open without a network, shows a reminder
 * pushed while no page is open, and is where a notification leads back to -
 * nothing else.
 *
 * It keeps the shell - the document, the script, the stylesheet, the pictures -
 * as last fetched, and hands it out when the installation cannot be reached.
 * Everything is asked of the network first, so a page that can reach the
 * installation always gets today's files; the copy is only for the moment it
 * cannot.
 *
 * Nothing the API answers is kept or handed out from here, ever. Who somebody is,
 * what they booked, what a balance says: all of it is asked fresh or not shown,
 * because a figure from an hour ago shown as if it were now is worse than none.
 * Bookings made meanwhile wait in the page itself - see keepForLater in app.js.
 */
const SHELL = 'gts-shell-v1';

/** What a page needs to draw itself at all, kept from the first visit. */
const ESSENTIALS = ['/', '/app.js', '/app.css', '/theme.js', '/favicon.svg'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(SHELL).then((cache) => cache.addAll(ESSENTIALS)));

  // The new shell keeper takes over at once rather than at the next visit: what
  // it keeps is only ever a copy, and an older keeper has nothing to finish.
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(caches.keys()
    .then((names) => Promise.all(names.filter((name) => name !== SHELL).map((name) => caches.delete(name))))
    .then(() => self.clients.claim()));
});

// A reminder pushed by the installation - a stopwatch past its hours, an unbooked
// day - shown as its notification, whether or not a page is open. The tag is the
// page's own for the same reminder, so one the page showed itself is replaced
// rather than shown twice.
//
// One that arrives late was held by the push service for a browser that was
// closed - for up to the hour it is sent to live - and what it says may have
// stopped being true meanwhile: a stopwatch stopped on another device, a day
// booked since. So it is asked about again first, as the page and the app ask
// about what they show, and one that no longer holds takes the notification shown
// for it before away with it.
self.addEventListener('push', (event) => {
  const message = event.data ? event.data.json() : {};
  if (!message.title) return;

  event.waitUntil(stillHolds(message).then(async (holds) => {
    if (holds) {
      await self.registration.showNotification(message.title, { body: message.body, tag: message.tag });

      return;
    }

    const shown = await self.registration.getNotifications({ tag: message.tag });
    shown.forEach((notification) => notification.close());
  }));
});

/** How late a pushed reminder may arrive and still be shown unasked: the installation confirmed it as it sent it. */
const PUSHED_FRESH = 5 * 60 * 1000;

/**
 * What the installation is asked about each kind of reminder, and whether its
 * answer bears out what the reminder is about: the moment the stopwatch started -
 * as a moment rather than as text, because the answer may write it with another
 * zone's offset than the push does - and the day with nothing booked.
 */
const ASKED_AGAIN = {
  'gts-stopwatch': ['/api/v1/me/timer', (answer, about) => Boolean(answer?.running)
    && Date.parse(answer.startedAt) === Date.parse(about)],
  'gts-unbooked': ['/api/v1/me/notices', (answer, about) => answer?.unbookedDay === about],
};

/** Whether a pushed reminder still holds: at once where it is fresh, and otherwise as the installation answers now. */
async function stillHolds(message) {
  const at = Date.parse(message.at);
  if (!Number.isFinite(at) || Date.now() - at <= PUSHED_FRESH) return true;

  const [path, bearsOut] = ASKED_AGAIN[message.tag] ?? [];
  if (!path) return false;

  try {
    const response = await fetch(path, { credentials: 'same-origin', headers: { Accept: 'application/json' } });
    if (!response.ok) return false;

    const answer = await response.json();

    return bearsOut(answer?.data, message.about);
  } catch {
    return false;
  }
}

// A notification - a stopwatch run past the hours its owner chose - leads back
// to the page: the window already open where there is one, a new one otherwise.
self.addEventListener('notificationclick', (event) => {
  event.notification.close();

  event.waitUntil(self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    .then((windows) => (windows.length > 0 ? windows[0].focus() : self.clients.openWindow('/'))));
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Only reads of this installation's own shell. The API, a write, and anything
  // from elsewhere go to the network exactly as they would without this.
  if (request.method !== 'GET' || url.origin !== self.location.origin || url.pathname.startsWith('/api/')) {
    return;
  }

  // A page is one document whatever its address says: the interface is a single
  // page, and the server answers every path with it.
  const key = request.mode === 'navigate' ? '/' : request;

  event.respondWith(fetch(request)
    .then((response) => {
      if (response.ok && response.type === 'basic') {
        const copy = response.clone();

        event.waitUntil(caches.open(SHELL).then((cache) => cache.put(key, copy)));
      }

      return response;
    })
    .catch(() => caches.match(key).then((kept) => kept ?? Response.error())));
});
