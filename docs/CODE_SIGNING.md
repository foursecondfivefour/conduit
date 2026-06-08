# Code signing (optional)

Conduit releases are published **unsigned** by default. Windows SmartScreen may show a warning on first run or after in-app updates.

## Sign with signtool

1. Obtain an Authenticode certificate (EV recommended for immediate SmartScreen reputation).
2. Install the certificate in the Windows certificate store or use a `.pfx` file.
3. Sign both binaries before publishing:

```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\conduit.exe
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\conduit-updater.exe
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\Conduit-Setup-1.2.0.exe
```

4. Verify:

```powershell
signtool verify /pa build\conduit.exe
```

## CI integration

`.github/workflows/release.yml` includes a commented optional step gated on `secrets.WINDOWS_CERT_BASE64`. Decode the PFX in CI only on protected branches and never commit certificates to the repository.
