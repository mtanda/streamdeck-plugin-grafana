# StreamDeck Grafana Plugin

A StreamDeck plugin that displays Prometheus/Grafana metrics with visual status indicators.

## Features

- **Real-time Prometheus Monitoring**: Query Prometheus endpoints and display metric values
- **Visual Status Indicators**: Color-coded backgrounds based on configurable thresholds
- **Automatic Updates**: Metrics refresh every 60 seconds
- **Instant Configuration Updates**: Changes to settings are applied immediately

## Status Colors

The plugin uses a traffic light color scheme to indicate metric status:

- 🟢 **Green**: Normal (below warning threshold)
- 🟠 **Orange**: Warning (above warning threshold, below critical threshold)
- 🔴 **Red**: Critical (above critical threshold)

## Configuration

### Required Settings

- **Prometheus Endpoint**: The URL of your Prometheus server (e.g., `https://prometheus.example.com`)
- **Username**: Authentication username for Prometheus
- **Password**: Authentication password for Prometheus
- **Query**: PromQL query to execute (e.g., `cpu_usage_percent`)

### Optional Settings

- **Threshold**: Comma-separated warning and critical thresholds (e.g., `60,80`)
  - First value: Warning threshold (orange color)
  - Second value: Critical threshold (red color)
  - Single value: Only warning threshold (e.g., `75`)
  - Empty: All values shown in green

## Usage

1. Drag the "Grafana Stat" action to a key on your StreamDeck
2. Configure the Prometheus connection settings
3. Set your PromQL query
4. Optionally configure thresholds for status indicators
5. The key will display the current metric value with appropriate color coding

## Building from Source

### Prerequisites

- Go 1.19 or later
- Git

### Build Steps

```bash
git clone https://github.com/mtanda/streamdeck-plugin-grafana.git
cd streamdeck-plugin-grafana
make build
```

The built plugin will be available in the `build/` directory.
