'use strict';

/**
 * Decides light or dark before the page is painted.
 *
 * This is a separate file loaded synchronously in <head> rather than part of
 * app.js, for two reasons. It has to run before the first paint or the page
 * flashes in the wrong colours, and the Content-Security-Policy forbids inline
 * script - so the usual one-liner in a <script> tag is not available here.
 *
 * Keep it small: everything in this file blocks rendering.
 */
(function applyThemeEarly() {
  /**
   * Where the choice is kept. Appearance belongs to the device rather than the
   * account: someone who wants dark on a laptop at night wants it whoever is
   * signed in.
   */
  var STORAGE_KEY = 'timepulse.theme';

  /**
   * Moves what this page kept on the device while the project was called
   * Go-Time-Recording to the names it reads now - an appearance chosen, a form
   * half filled in, the screen last open - or the first load after the update
   * would leave all of it under names nothing reads any more.
   *
   * Here because this is the first script on the page and the first to read one
   * of them. A value already kept under today's name wins, because it was written
   * later. The bookings kept without a connection are not moved here: another tab
   * may be sending them as this runs, so app.js takes them over under the lock
   * the sending is done under.
   * @param {Storage} storage
   */
  function adoptWhatWasKeptBeforeTheRename(storage) {
    var keys = [];
    var i;

    for (i = 0; i < storage.length; i++) keys.push(storage.key(i));

    for (i = 0; i < keys.length; i++) {
      var key = keys[i];
      if (!/^gtr[._]/.test(key) || key.indexOf('gtr_outbox_') === 0) continue;

      var renamed = 'timepulse' + key.slice('gtr'.length);
      if (storage.getItem(renamed) === null) storage.setItem(renamed, storage.getItem(key));

      storage.removeItem(key);
    }
  }

  try {
    adoptWhatWasKeptBeforeTheRename(localStorage);
    adoptWhatWasKeptBeforeTheRename(sessionStorage);
  } catch (e) {
    // Private browsing can refuse storage entirely, and then nothing was kept.
  }

  /**
   * Dark from dusk to dawn, in local time - that is what "evening" means to the
   * person looking at the screen.
   */
  var DARK_FROM_HOUR = 19;
  var DARK_UNTIL_HOUR = 7;

  /**
   * What "auto" means at a given hour.
   * @param {number} hour
   * @returns {'light'|'dark'}
   */
  function themeForHour(hour) {
    return hour >= DARK_FROM_HOUR || hour < DARK_UNTIL_HOUR ? 'dark' : 'light';
  }

  /** @returns {'auto'|'light'|'dark'} */
  function storedPreference() {
    try {
      var stored = localStorage.getItem(STORAGE_KEY);

      return stored === 'light' || stored === 'dark' ? stored : 'auto';
    } catch (e) {
      // Private browsing can refuse storage entirely; the default still works.
      return 'auto';
    }
  }

  /** @returns {'light'|'dark'} */
  function resolve(preference) {
    return preference === 'auto' ? themeForHour(new Date().getHours()) : preference;
  }

  var preference = storedPreference();

  document.documentElement.dataset.theme = resolve(preference);
  document.documentElement.dataset.themePreference = preference;

  applyStoredBranding();

  /**
   * Puts the instance's own name and mark on the page before it is painted.
   *
   * Both are configured, so both live in the database, so the page cannot know
   * either until it has asked - and asking is a request that finishes after the
   * document has been parsed and shown. Until then the tab reads "TimePulse"
   * and shows the shipped mark, whatever this installation calls itself. On a
   * reload that is a visible flicker back to a name nobody chose.
   *
   * The tab icon is no longer here: the server writes that into the document, at
   * an address carrying a fingerprint of what it answers with, so it is right on
   * the first paint without anything on this side.
   *
   * The title is written by the server too. This stays as the answer for the one
   * case the server cannot cover - a page restored from the browser's own cache,
   * where no request was made and the document is whatever it was last time.
   */
  function applyStoredBranding() {
    var stored;

    try {
      stored = JSON.parse(localStorage.getItem('timepulse.branding') || 'null');
    } catch (e) {
      return;
    }

    if (!stored || typeof stored !== 'object') return;

    if (typeof stored.title === 'string' && stored.title) {
      document.title = stored.title;
    }
  }

  // Exposed so app.js reuses these rules rather than restating them.
  window.timePulseTheme = {
    STORAGE_KEY: STORAGE_KEY,
    DARK_FROM_HOUR: DARK_FROM_HOUR,
    DARK_UNTIL_HOUR: DARK_UNTIL_HOUR,
    forHour: themeForHour,
    stored: storedPreference,
    resolve: resolve,
  };
})();
