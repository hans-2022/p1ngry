This fork was made for following reasons: 

a) I had some problems installing the original version on a Raspberry Pi
======
HAOS supervisor logs indicated: An unknown error occurred with app 3b571237_p1nrgy. Noticed it meant a missing ARM64 HAOS manifest / Docker image
Worked in the arm64 image. 
======
b) Afer starting p1ngry on arm64 pi (after fix) I noticed all P1 telegrams were logged to the supervisor log and the connection to mqtt/emqx did not work
======
Went ahead to implement a toggle in the config for this logging and 
Developed some extra config entries to flexibly use for MQTT / EMQX which are used in p1ngry
======

