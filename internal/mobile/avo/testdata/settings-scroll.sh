# Sent to Android's shell by both live benchmark harnesses.
set -eu
period=$(dumpsys SurfaceFlinger --latency 2>/dev/null | head -1)
case "$period" in ''|*[!0-9]*) ;; *) printf 'display_period_ns=%s\n' "$period" ;; esac
am start -W -a android.settings.SETTINGS >/dev/null
dumpsys gfxinfo com.android.settings reset >/dev/null
passes=${1:-1}
case "$passes" in 1|20) ;; *) exit 2 ;; esac
i=0
while [ "$i" -lt "$passes" ]; do
  for y in 1200 450 1200 450 1200; do
    input swipe 390 "$y" 390 "$((1650-y))" 350
    sleep 0.2
  done
  i=$((i+1))
done
stats=$(dumpsys gfxinfo com.android.settings | grep -E 'Total frames rendered|Janky frames:|95th percentile:|Pipeline=')
printf '%s\n' "$stats"
printf '%s\n' "$stats" | grep -Eq 'Total frames rendered: [1-9][0-9]*'
