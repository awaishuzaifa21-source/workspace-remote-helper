# Workspace Remote Helper

The small open-source program behind **[Workspace free remote support](https://huzaifaawais.me/workspace/free-remote-support-software/)**. A customer runs it so a technician can move the mouse and type on their computer from a web browser during one support session.

Workspace is a free ERP and remote support app with no paid plans. Read more at **[huzaifaawais.me/workspace](https://huzaifaawais.me/workspace/)**, or see how it compares in **[Free TeamViewer alternative](https://huzaifaawais.me/workspace/teamviewer-alternative/)**.

## What it does, and what it won't do

- Listens only on this computer (`127.0.0.1`) and only answers the Workspace website.
- Asks the person at the computer to approve control in a system dialog, every session.
- Stops when the web page says stop, when the window is closed, or after 20 seconds without contact.
- Never installs itself, starts on boot or runs hidden.

Screen sharing itself happens in the browser (WebRTC). The helper only handles mouse and keyboard input.

## Download

Ready-made builds for Windows, macOS and Linux are on the [Workspace remote support page](https://huzaifaawais.me/workspace/free-remote-support-software/).

## Build from source

Needs Go 1.24. No C compiler required.

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o workspace-helper.exe .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o workspace-helper-mac .
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o workspace-helper-linux .
```

| Platform | How input is sent |
|---|---|
| Windows | `SendInput` from user32 |
| macOS | CoreGraphics events (needs Accessibility permission) |
| Linux | XTest on X11 (`libX11`, `libXtst`) |

## Author

Made by [Huzaifa Awais](https://huzaifaawais.me/). More free things I build: [free online tools](https://huzaifaawais.me/tools/) and a [free lead finder](https://huzaifaawais.me/free-lead-finder/).

## License

MIT
