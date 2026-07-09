# Home Assistant Integration

This guide shows how to expose the remaining time from PulseOrPerish as a sensor in Home Assistant.

It uses the [RESTful Sensor](https://www.home-assistant.io/integrations/sensor.rest/) integration with the public `GET /status` endpoint.

This guide only covers the sensor configuration and the templating needed to turn the JSON response into a Home Assistant-friendly value. It does not cover installing or enabling the REST integration itself.

## PulseOrPerish Status Payload

The `GET /status` endpoint returns a JSON object similar to this:

```json
{
  "lastProofAt": "2026-07-09T18:00:00Z",
  "nextDeletion": "2026-07-10T18:00:00Z",
  "timeRemaining": "23h58m12s",
  "timeRemainingMinutes": 1439,
  "overdue": false,
  "dryRun": false
}
```

Relevant fields for Home Assistant:

- `timeRemainingMinutes` is the numeric value intended for the Home Assistant sensor state.
- `timeRemaining` is the original Go duration.
- `nextDeletion` and `lastProofAt` are useful as sensor attributes.
- `overdue` and `dryRun` can also be exposed as attributes.

## REST Sensor Configuration

Add a REST sensor that polls the public `/status` endpoint and uses `timeRemainingMinutes` as the entity state.

```yaml
sensor:
  - platform: rest
    name: PulseOrPerish Remaining Time
    unique_id: pulseorperish_remaining_time
    device_class: duration
    resource: http://YOUR_HOST:8080/status
    method: GET
    scan_interval: 300 # in seconds
    unit_of_measurement: "min"
    value_template: "{{ value_json.timeRemainingMinutes }}"
    json_attributes:
      - timeRemaining
      - nextDeletion
      - lastProofAt
      - overdue
      - dryRun
```
