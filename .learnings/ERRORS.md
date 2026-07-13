# Errors log

## 2026-07-13 msp-seed field/method name clash

- **Symptom**: compile error `field and method with the same name peers/packets`
- **Cause**: `hub.peers` map field vs `func (h *hub) peers` handler
- **Fix**: rename handlers to `handlePeers`, `handlePackets`, etc.

## 2026-07-13 Start-Process denied

- **Symptom**: Windows denied `Start-Process msp-seed.exe`
- **Workaround**: run seed via shell tool `background: true` instead of Start-Process.
