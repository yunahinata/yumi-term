# yumi-term 🌸

> A fresh take on terminal emulators. What if your command line felt like a modern messenger?

![yumi-term banner](./image.png)

**yumi-term** is a minimalist TUI terminal wrapper built with Go and Bubble Tea. It reimagines the classic terminal layout by structuring your session like a chat feed: your commands fly to the right inside clean bubbles, while system responses stay on the left.

## ✨ Features
- **Messenger-like Layout:** Your inputs are on the right, system outputs are on the left.
- **Asynchronous Execution:** Heavy commands won't freeze the UI. Scroll and navigate freely while tasks run in the background.
- **Smart Directory Tracking:** Fully supports `cd` commands and dynamically updates your current working directory in the status bar.
- **Catppuccin Mocha Palette:** Beautiful, eye-friendly pastel colors out of the box.
- **Mouse & Keyboard Scrolling:** Navigate your logs easily with the mouse wheel or `↑`/`↓` arrows.

## 🚀 Quick Start

```bash
go run main.go
