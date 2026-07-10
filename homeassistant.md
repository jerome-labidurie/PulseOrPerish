# Home Assistant Integration

This guide shows how to expose the remaining time from PulseOrPerish as a sensor in Home Assistant.

![HA](./img/homeassistant.png)

It uses the [RESTful Sensor](https://www.home-assistant.io/integrations/sensor.rest/) integration with the public `GET /status` endpoint.

This guide only covers the sensor configuration and the templating needed to turn the JSON response into a Home Assistant-friendly value.

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

## Automation: Alerts at 10d, 5d and 1d

The example below uses one automation with three triggers and shared actions.

Each alert sends:
- a notification to your mobile app
- a persistent notification in the Home Assistant interface

Replace `notify.mobile_app_your_phone` with your real mobile notify entity.

```yaml
automation:
  - id: pulseorperish_notify_before_deletion
    alias: PulseOrPerish - Notify before deletion
    description: Sends mobile + HA UI notifications at T-10, T-5 and T-1 days.
    mode: single
    trigger:
      - platform: numeric_state
        id: t_minus_10d
        entity_id: sensor.pulseorperish_remaining_time
        below: 14400
        above: 7200
      - platform: numeric_state
        id: t_minus_5d
        entity_id: sensor.pulseorperish_remaining_time
        below: 7200
        above: 1440
      - platform: numeric_state
        id: t_minus_1d
        entity_id: sensor.pulseorperish_remaining_time
        below: 1440
        above: 0
    action:
      - variables:
          days_left: >-
            {% set map = {'t_minus_10d': 10, 't_minus_5d': 5, 't_minus_1d': 1} %}
            {{ map.get(trigger.id, '?') }}
          severity_label: >-
            {% set map = {'t_minus_10d': 'warning', 't_minus_5d': 'warning', 't_minus_1d': 'critical warning'} %}
            {{ map.get(trigger.id, 'warning') }}
      - action: notify.send_message
        target:
          entity_id: notify.mobile_app_your_phone
        data:
          message: >-
            PulseOrPerish: {{ days_left }} day{{ '' if days_left in ['1', 1] else 's' }} remaining before deletion.
            Next deletion: {{ state_attr('sensor.pulseorperish_remaining_time', 'nextDeletion') }}
      - action: notify.persistent_notification
        data:
          title: PulseOrPerish {{ severity_label }}
          message: >-
            {{ days_left }} day{{ '' if days_left in ['1', 1] else 's' }} remaining before deletion.
            Next deletion: {{ state_attr('sensor.pulseorperish_remaining_time', 'nextDeletion') }}
```
