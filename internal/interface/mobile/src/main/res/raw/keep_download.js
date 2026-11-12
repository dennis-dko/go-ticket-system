// Lets the web interface's files be saved from the app.
//
// The page saves a file as a browser does: a blob, a link to it that is
// clicked, and the blob released at once. A WebView saves none of that by
// itself, nor can the blob be read back once it is released. So the blob is kept
// when its link is made, and a click on such a link hands it to the app, which
// asks where to save it.
(function () {
  if (window.goTicketSystemKeepsDownloads) return;
  window.goTicketSystemKeepsDownloads = true;

  const blobs = new Map();
  const create = URL.createObjectURL;

  URL.createObjectURL = function (blob) {
    const url = create.call(URL, blob);
    if (blob instanceof Blob) blobs.set(url, blob);

    return url;
  };

  const click = HTMLAnchorElement.prototype.click;

  HTMLAnchorElement.prototype.click = function () {
    const blob = blobs.get(this.href);
    if (!blob || !this.download) return click.apply(this, arguments);

    blobs.delete(this.href);

    const name = this.download;
    const reader = new FileReader();

    reader.onload = () => GoTicketSystemFiles.save(String(reader.result).split(',')[1] || '', name);
    reader.readAsDataURL(blob);
  };
})();
