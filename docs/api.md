# API
List of supported endpoints by the application:



- `GET /api/v1/config/bridge`: Returns as JSON object the full configuration of the [bridge.yml](configuration.md) where the keys are in `camelCase` format.
- `GET /api/v1/min-amounts`: Returns as JSON object the current min-amounts per asset per network in the following format:
```json
{
  "networkId": {
    "assetIdOrAddress": "min-amount"
  }
}
```
Example:
```json
{
  "295": {
    "HBAR": "20736132711",
    "0.0.26056684": "144956212352"
  }, 
  "1": {
    "0x14ab470682Bc045336B1df6262d538cB6c35eA2A": "20736132711",
    "0xac3211a5025414Af2866FF09c23FC18bc97e79b1": "1449562123521537231600"
  }, 
  "137": {
    "0x1646C835d70F76D9030DF6BaAeec8f65c250353d": "20736132711"
  }
}
```