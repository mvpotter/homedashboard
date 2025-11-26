## ESPHome

### Apply config

```bash
cd esphome
esphome run e1001.yaml
```

### Connect to read logs

```bash
cd esphome
esphome logs e1001.yaml
```

## App

### Deploy

```bash
make build/homedashboard
make production/deploy/homedashboard
```
