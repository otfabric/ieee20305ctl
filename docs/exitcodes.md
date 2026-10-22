# Exit Codes

| Code | Constant         | Meaning                                         |
|------|------------------|-------------------------------------------------|
| 0    | `ExitOK`         | Successful execution                            |
| 1    | `ExitError`      | General runtime error                           |
| 2    | `ExitUsage`      | Incorrect command usage or invalid arguments     |
| 3    | `ExitConnect`    | Connection or TLS handshake failure              |
| 4    | `ExitCertificate`| Certificate error (missing, invalid, or expired) |
| 5    | `ExitInvalid`    | A document does not satisfy the schema (`validate`) |
