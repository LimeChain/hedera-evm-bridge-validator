# Metrics

The following table lists the currently available metrics in Prometheus/Grafana with their short description.

| Name                                                                                         | Description                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|----------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `validators_participation_rate`                                                              | Participation rate: Track validators' activity in %.                                                                                                                                                                                                                                                                                                                                                                                         |
| `fee_account_amount`                                                                         | Fee account amount.
| `bridge_account_amount`                                                                      | Bridge account amount.
| `operator_account_amount`                                                                    | Operator account amount.
| `${TOKEN_TYPE}_${NATIVE_NETWORK}_${NETWORK}_total_supply_asset_id_${ASSET_ID}`               | The Total Supply of the wrapped asset with a given ID. The prefix is`${TOKEN_TYPE}_${NATIVE_NETWORK}`, where `${TOKEN_TYPE}` is `Native` or `Wrapped`, `${NATIVE_NETWORK}` is the name of the native network for a given asset, and `${NETWORK}` the name of the network. The suffix of the metric is `_total_supply_asset_id_${ASSET_ID}`.
| `${TOKEN_TYPE}_${NATIVE_NETWORK}_${NETWORK}_balance_asset_id_${ASSET_ID}`                    | The Balance of the native asset with a given ID. The prefix is `${TOKEN_TYPE}_${NATIVE_NETWORK}`, where `${TOKEN_TYPE}` is `Native` or `Wrapped`, `${NATIVE_NETWORK}` is the name of the native network for a given asset, and `${NETWORK}` the name of the network. The suffix of the metric is `_balance_asset_id_${ASSET_ID}`.
| `${TOKEN_TYPE}_${SOURCE_NETWORK}_to_${TARGET_NETWORK}_${TRANSACTION_ID}_majority_reached`    | Is metric which gives info about `majority_reached` (are all signatures are collected) for the given token type (Native or Wrapped), source and target networks and transaction id. 
| `${TOKEN_TYPE}_${SOURCE_NETWORK}_to_${TARGET_NETWORK}_${TRANSACTION_ID}_fee_transferred`     | Is metric which gives info about `fee_transferred` (is the fee transferred between the validators) for the given token type (Native or Wrapped), source and target networks and transaction id. 
| `${TOKEN_TYPE}_${SOURCE_NETWORK}_to_${TARGET_NETWORK}_${TRANSACTION_ID}_user_get_his_tokens` | Is metric which gives info about `user_get_his_tokens` (does the user made the transaction to get his tokens after the transfer) for the given token type (Native or Wrapped), source and target networks and transaction id. 