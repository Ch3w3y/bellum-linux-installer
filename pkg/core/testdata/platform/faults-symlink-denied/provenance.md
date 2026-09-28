# faults-symlink-denied — synthetic fault fixture

Origin: constructed from the SOF-4 fault matrix. No real hardware capture;
never a real capture. Covers: denied os-release with no silent fallback
replacement, cyclic and escaping Steam root links (escape never reaches host
content), malformed ICD manifest, headless session, unknown Steam ownership
(EUID withheld), software renderer.
