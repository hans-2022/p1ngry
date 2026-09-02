This fork was made for the following reasons:

### 1. ARM64 / Raspberry Pi installation

I had some problems installing the original version on a Raspberry Pi.

The HAOS Supervisor logs indicated:

`An unknown error occurred with app 3b571237_p1nrgy`

I discovered that the original Docker image was missing a suitable ARM64 HAOS manifest.

This fork was therefore created to provide a working ARM64 image.

### 2. P1 telegram logging and MQTT / EMQX configuration

After getting p1ngry running on the ARM64 Raspberry Pi, I noticed that all P1 telegrams were being logged to the HAOS Supervisor log, and the connection to MQTT / EMQX was not working.

I therefore:

- Implemented a configuration toggle to enable or disable the P1 telegram logging.
- Added extra configuration entries for flexible MQTT / EMQX configuration.
- Connected these configuration settings to the p1ngry MQTT functionality.

The goal is to make p1ngry easier to run on a Raspberry Pi with HAOS and to provide more flexible MQTT / EMQX configuration.
