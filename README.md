# Chargebot

A battery charging minder.

## Motivation

I have a small power bank for emergencies; it's a [Goal Zero Yeti 200X](https://www.goalzero.com/collections/portable-power-stations/products/goal-zero-yeti-200x-portable-power-station), which I'd like to keep topped up and ready to use, but I don't want it plugged into the wall forever.

Using a smart plug and this tool, every six days the plug is turned on, waits until the battery appears to be charged, and then turns it off.

## Usage

This is based off a SwitchBot Plug that has been re-flashed to Tasmota, and is talking to an MQTT broker. Any plug that reports power values to MQTT should work, but I've only tested with the SwitchBot.

I run it like this, from a raspberry pi:

```bash
$ ./chargebot \
  -broker tcp://192.168.1.5:1883 \
  -mt tele/tasmota_AEE61C/SENSOR \
  -ct cmnd/tasmota_AEE61C/Power
```

You'll need the monitoring and control topics for your plug, which you can find in the Tasmota web interface. If your plug returns some other kind of JSON payload you may need to adjust the `TasmotaStatus` struct.
