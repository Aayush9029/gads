<p align="center">
  <img src="assets/icon.png" width="128" alt="gads">
  <h1 align="center">gads</h1>
  <p align="center">Query, report on, and safely change Google Ads accounts from the terminal</p>
</p>

<p align="center">
  <a href="https://github.com/Aayush9029/gads/releases/latest"><img src="https://img.shields.io/github/v/release/Aayush9029/gads" alt="Release"></a>
  <a href="https://github.com/Aayush9029/gads/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Aayush9029/gads" alt="License"></a>
</p>

<p align="center">
  <img src="assets/demo.gif" alt="gads demo" width="800">
</p>

## Install

```bash
brew install aayush9029/tap/gads
```

Or tap first:

```bash
brew tap aayush9029/tap
brew install gads
```

## Usage

```bash
gads profile set --name work --customer 123-456-7890
gads auth login --profile work
gads accounts tree --profile work --output table
gads query run --profile work --file report.gaql --stream --output jsonl
gads campaign create --profile work --body-file campaign.json
```

Write commands produce a local plan unless you pass `--validate` or `--execute`. New campaigns are always created paused.

See the [usage guide](docs/USAGE.md), [command reference](docs/commands/gads.md), and [Google Ads API map](docs/OFFICIAL_GOOGLE_ADS_DOCS.md).
