# Hexone Privacy Policy

Effective date: July 13, 2026

Hexone is a local, open-source file manager published by Ramūnas Cvirka. This policy explains what information Hexone accesses, how that information is used and stored, and the choices available to users.

## Summary

Hexone does not include telemetry, analytics, advertising, tracking, crash-reporting services, user accounts, or a publisher-operated cloud service. The publisher does not receive or collect information through the application.

Hexone processes files, paths, settings, and connection information locally when needed to perform actions requested by the user. When a user connects to an SSH/SFTP server, Hexone communicates only with the server selected by that user.

## Information Hexone accesses

Hexone may access the following information as part of its functionality:

- local file and directory names, paths, metadata, and contents selected or navigated by the user;
- remote file and directory information accessed through a user-configured SSH/SFTP connection;
- application settings, favorites, command history, file associations, and SSH connection profiles;
- SSH host names or addresses, ports, user names, key-file paths, passwords, and private-key passphrases entered by the user; and
- text, file paths, or selections supplied to commands and external applications explicitly configured or launched by the user.

Hexone uses this information only to provide the requested file-management, viewing, terminal, command, and SSH/SFTP functionality.

## Local storage and security

Application settings and connection profile metadata are stored locally in `hexone.yaml`. Depending on the installation type, this file is stored in the application's platform-specific configuration directory, beside the portable executable, or in the Microsoft Store package's `LocalState` directory.

SSH passwords and private-key passphrases are not written to `hexone.yaml`. Saved secrets are stored using the operating system's credential facility:

- Windows Credential Manager on Windows;
- Keychain on macOS; or
- a compatible Secret Service provider on Linux.

Hexone does not initiate cloud backups. The operating system or other software selected by the user may back up local application data or credential-store data according to that software's settings and privacy terms.

## Network communication and disclosure

Hexone does not transmit information to the publisher or to an analytics or advertising provider.

Network communication occurs when the user initiates an SSH/SFTP connection. Connection credentials, commands, directory information, and file contents required for the requested operation are transmitted through the encrypted SSH connection to the server specified by the user. Hexone does not control that server; its administrator or service provider is responsible for the server's storage, logging, retention, and privacy practices.

Hexone can also launch commands and external applications selected or configured by the user. Those programs run with the user's permissions and may have their own network behavior and privacy policies. Their activity is not data collection by Hexone.

Hexone does not sell personal information and does not disclose it except as directed by the user through these connections, commands, or external applications, or when required by applicable law.

## User controls and retention

Users control which files, folders, remote servers, commands, and external applications Hexone accesses.

- Saved SSH profiles can be edited or removed in Hexone. Removing a profile removes its associated saved secret from the operating system credential store.
- Other locally stored settings can be changed in Hexone or removed by deleting the Hexone configuration files.
- Files copied to or stored on a remote server must be managed or deleted on that server.

Uninstall behavior varies by platform. To ensure that saved SSH credentials are removed, delete the corresponding SSH profiles in Hexone before uninstalling. Users can also inspect and remove remaining entries directly through their operating system's credential-management tools.

## Children's privacy

Hexone is a general-purpose file-management utility and is not directed specifically at children. Hexone does not knowingly collect personal information from children or any other users.

## Changes to this policy

This policy may be updated when Hexone's functionality or data practices change. Updates will be published in this repository with a revised effective date.

## Contact

For privacy questions or requests, open an issue at:

https://github.com/ramunas-cvirka/hexone/issues
