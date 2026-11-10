#!/usr/bin/env bash
# The Android app's pictures in the README, taken from the app rather than drawn.
#
# task readme:pictures:app runs this from the repository's root, where adb and an
# emulator are - inside WSL on Windows - once it has built what this needs: the
# web binary for Linux in bin/linux and the app in bin/mobile. It serves a fresh
# instance beside the emulator, which reaches it as 10.0.2.2, books a few working
# days into it through the API, installs the app signed with a throwaway key -
# never the release key - and photographs four screens into doc/image:
#
#   app-connect    the screen that connects the app, as a new installation shows it
#   app-scan       the camera screen that reads a token's code
#   app-stopwatch  the web interface in the app: the time entries, a stopwatch running
#   app-menu       the web interface's menu, opened over the page
#
# test/picture_test.go holds the README to exactly the names this shoots.
set -euo pipefail

sdk=${ANDROID_SDK_ROOT:-$HOME/android/sdk}
jdk=${JAVA_HOME:-$HOME/android/jdk}
port=${GTR_CAMERA_PORT:-8091}
package=io.github.dennisdko.gotimerecording
apk=bin/mobile/outputs/apk/release/go-time-recording-mobile-release-unsigned.apk
into=doc/image

export PATH="$jdk/bin:$sdk/platform-tools:$PATH"

work=$(mktemp -d)
server=""

cleanup() {
  if [ -n "$server" ]; then kill "$server" 2> /dev/null || true; fi
  adb shell wm size reset > /dev/null 2>&1 || true
  adb shell wm density reset > /dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

fail() {
  echo "android.sh: $*" >&2
  exit 1
}

[ -f bin/linux/go-time-recording-web ] || fail "no bin/linux/go-time-recording-web: run task readme:pictures:app"
[ -f "$apk" ] || fail "no $apk: run task readme:pictures:app"
adb get-state > /dev/null 2>&1 || fail "adb reaches no device: start an emulator first"

# ------------------------------------------------------------------ the phone

# English, as the README is. The app's own screens follow the phone's language
# and the web interface the browser's, which is the phone's as well. Changing it
# takes root, which an emulator's google_apis image allows, and a restart, after
# which the property that says the phone has booted starts again from nothing.
if [ "$(adb shell getprop persist.sys.locale | tr -d '\r')" != en-US ]; then
  adb root > /dev/null
  adb wait-for-device
  adb shell setprop persist.sys.locale en-US
  adb reboot
  adb wait-for-device
fi

for _ in $(seq 1 120); do
  [ "$(adb shell getprop sys.boot_completed 2> /dev/null | tr -d '\r')" = 1 ] && break
  sleep 2
done

# Half the pixels each way at half the density, which is the same 411 dp and so
# the same layout: a picture the README shows a quarter of its width needs no
# more, and a camera's picture of a room is a large PNG at full size.
adb shell wm size 540x1200
adb shell wm density 210

# The day the pictures show is the phone's: the installation keeps its zone, so
# the stopwatch and the entries fall on the day the status bar shows.
zone=$(adb shell getprop persist.sys.timezone | tr -d '\r')
[ -n "$zone" ] || zone=UTC

day() { TZ="$zone" date -d "$1" +%F; }

# --------------------------------------------------------- the installation

mkdir -p "$work/configs"
cp bin/linux/go-time-recording-web "$work/"
chmod +x "$work/go-time-recording-web"
tr -d '\r' < cmd/configs/.env > "$work/configs/.env"

(cd "$work" && exec env APP_ENV= DB_DIALECT=sqlite DB_NAME="$work/gtr.db" HTTP_PORT="$port" \
  METRICS_PORT=$((port + 1)) LOG_LEVEL=WARN ./go-time-recording-web > "$work/log" 2>&1) &
server=$!

for _ in $(seq 1 60); do
  curl -fsS "http://127.0.0.1:$port/.well-known/alive" > /dev/null 2>&1 && break
  sleep 1
done
curl -fsS "http://127.0.0.1:$port/.well-known/alive" > /dev/null 2>&1 || {
  tail -n 20 "$work/log" >&2
  fail "the installation did not start"
}

base="http://127.0.0.1:$port/api/v1"
jar="$work/jar"

# A session's changes carry its CSRF cookie back in X-CSRF-Token, and any answer
# sets the cookie, so a session starts with a read.
session() {
  rm -f "$jar"
  curl -fsS -c "$jar" "http://127.0.0.1:$port/" > /dev/null
}

signed_in() {
  curl -fsS -b "$jar" -c "$jar" -X "$1" -H 'Content-Type: application/json' \
    -H "Origin: http://127.0.0.1:$port" \
    -H "X-CSRF-Token: $(awk '$6 == "gtr_csrf" { print $7 }' "$jar" | tail -n 1)" \
    "$base$2" ${3:+-d "$3"}
}

# As the owner of the token the phone is connected with, the way a program asks.
as_robin() {
  curl -fsS -X "$1" -H 'Content-Type: application/json' -H "Authorization: Bearer $token" \
    "$base$2" ${3:+-d "$3"}
}

session
signed_in POST /auth/login '{"email":"admin@local","password":"changeme123"}' > /dev/null
signed_in PUT /me/password '{"currentPassword":"changeme123","newPassword":"a-camera-password-1"}' > /dev/null
session
signed_in POST /auth/login '{"email":"admin@local","password":"a-camera-password-1"}' > /dev/null
signed_in PUT /settings/timezone "{\"timezone\":\"$zone\"}" > /dev/null
signed_in POST /users \
  '{"name":"Robin Keller","email":"robin@example.com","role":"user","password":"robin-password-1"}' > /dev/null

session
signed_in POST /auth/login '{"email":"robin@example.com","password":"robin-password-1"}' > /dev/null
token=$(signed_in POST /me/tokens '{"name":"Phone"}' | grep -o -E '"secret":"[^"]+"' | cut -d'"' -f4)
[ -n "$token" ] || fail "the installation made no token"

# Robin's last few days, in the projects and words of the browser's pictures, and
# the walk through recorded as seen, so it does not open over the pictures.
as_robin PUT /me/tour '{"seen":true}' > /dev/null

project() {
  as_robin POST /projects "{\"name\":\"$1\",\"description\":\"$2\",\"startDate\":\"$(day '400 days ago')\",
    \"status\":\"active\",\"billable\":$3,\"hourlyRate\":$4,\"currency\":\"EUR\"}" |
    grep -o -E '"id":[0-9]+' | head -n 1 | cut -d: -f2
}

web=$(project 'Website relaunch' 'New shop front and checkout' true 95)
app=$(project 'Mobile app' 'Release 2.4 for iOS and Android' true 110)
support=$(project 'Customer support' 'Second-level tickets' true 70)
tooling=$(project 'Internal tooling' 'Build pipeline and deployment' false null)

book() {
  as_robin POST /timesheets \
    "{\"projectId\":$1,\"date\":\"$2\",\"durationHours\":$3,\"description\":\"$4\"}" > /dev/null
}

back=0
for plan in 1 2 3; do
  back=$((back + 1))
  while [ "$(TZ="$zone" date -d "$back days ago" +%u)" -gt 5 ]; do back=$((back + 1)); done
  on=$(day "$back days ago")

  case $plan in
    1)
      book "$web" "$on" 3.5 '#design Landing page layout'
      book "$app" "$on" 2.5 '#review Release 2.4 pull requests'
      book "$support" "$on" 1.5 '#support Ticket queue'
      ;;
    2)
      book "$app" "$on" 4 '#development Offline sync'
      book "$web" "$on" 2 '#meeting Sprint planning'
      book "$support" "$on" 2.25 '#support Escalations'
      ;;
    3)
      book "$web" "$on" 5 '#development Checkout flow'
      book "$tooling" "$on" 1 '#devops CI pipeline'
      ;;
  esac
done

today=$(day today)
book "$tooling" "$today" 0.5 '#meeting Stand-up'
book "$support" "$today" 1.5 '#support Ticket queue'
book "$app" "$today" 2 '#review Code review'

as_robin POST /me/timer "{\"projectId\":$web,\"description\":\"#design Hero section\"}" > /dev/null
as_robin PUT /me/timer "{\"startedAt\":\"$(date -u -d '80 minutes ago' +%Y-%m-%dT%H:%M:%SZ)\"}" > /dev/null

# ------------------------------------------------------------------- the app

# A key of its own each run, so the app is put on afresh: Android refuses an
# update signed with a key other than the installed one's.
tools=$(printf '%s\n' "$sdk"/build-tools/* | sort -V | tail -n 1)
keytool -genkeypair -keystore "$work/throwaway.keystore" -storepass throwaway -keypass throwaway \
  -alias throwaway -keyalg RSA -keysize 2048 -validity 1 -dname CN=throwaway > /dev/null 2>&1
"$tools/zipalign" -f 4 "$apk" "$work/aligned.apk"
"$tools/apksigner" sign --ks "$work/throwaway.keystore" --ks-pass pass:throwaway \
  --out "$work/app.apk" "$work/aligned.apk"
adb uninstall "$package" > /dev/null 2>&1 || true
adb install "$work/app.apk" > /dev/null
adb shell pm grant "$package" android.permission.CAMERA

# where finds the first node on the screen whose text or description is $1 and
# answers its bounds, waiting up to thirty seconds for it. A node of the page that
# is off the screen has bounds of nothing, and is not on the screen.
where() {
  for _ in $(seq 1 15); do
    adb shell uiautomator dump /sdcard/ui.xml > /dev/null 2>&1 || true
    local bounds
    bounds=$(adb shell cat /sdcard/ui.xml 2> /dev/null |
      grep -o -E "(text|content-desc)=\"$1\"[^>]*bounds=\"\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]\"" |
      grep -o -E '\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]' | grep -v -F '[0,0][0,0]' | head -n 1 || true)
    if [ -n "$bounds" ]; then
      echo "$bounds"
      return
    fi
    sleep 2
  done
}

await() { [ -n "$(where "$1")" ] || fail "\"$1\" never appeared on the screen"; }

tap() {
  local bounds x1 y1 x2 y2
  bounds=$(where "$1")
  [ -n "$bounds" ] || fail "\"$1\" never appeared on the screen"
  read -r x1 y1 x2 y2 <<< "$(echo "$bounds" | tr -c '0-9' ' ')"
  adb shell input tap $(((x1 + x2) / 2)) $(((y1 + y2) / 2))
}

# shoot writes the screen as it is to doc/image under the picture's name.
shoot() {
  sleep 2
  adb exec-out screencap -p > "$into/$1.png"
  echo "$1.png: $(($(stat -c %s "$into/$1.png") / 1024)) KB"
}

adb shell monkey -p "$package" -c android.intent.category.LAUNCHER 1 > /dev/null 2>&1
await 'Connect to your installation'
shoot app-connect

tap 'Scan the code'
await "Scan the token's code"

# An emulator renders its camera's room in software, and the first frames take
# a few seconds; until they come the window in the veil is black, and a screen
# that is nearly all black makes a small PNG.
for _ in $(seq 1 20); do
  adb exec-out screencap -p > "$work/scan.png"
  [ "$(stat -c %s "$work/scan.png")" -gt 60000 ] && break
  sleep 2
done
shoot app-scan
tap 'Cancel'

# The code's link, the way the phone's camera hands it over; the app offers it
# and connects only once Connect is pressed.
adb shell "am start -a android.intent.action.VIEW -d 'go-time-recording://connect?url=http%3A%2F%2F10.0.2.2%3A$port&token=$token'" > /dev/null
tap 'Connect'
tap 'Menu'
tap 'Time entries'
await 'Stop and book'
shoot app-stopwatch

tap 'Menu'
await 'My account'
shoot app-menu
