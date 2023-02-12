KEY="mykey"
CHAINID="ttestchain_9100-1"
MONIKER="localtestnet"
KEYRING="test"
KEYALGO="eth_secp256k1"
LOGLEVEL="info"
# to trace evm
#TRACE="--trace"
TRACE=""

# validate dependencies are installed
command -v jq > /dev/null 2>&1 || { echo >&2 "jq not installed. More info: https://stedolan.github.io/jq/download/"; exit 1; }

# used to exit on first error (any non-zero exit code)
set -e

# Reinstall daemon
rm -rf ~/.testd*
make install

# Set moniker and chain-id for Testchain (Moniker can be anything, chain-id must be an integer)
testd init $MONIKER --chain-id $CHAINID

# Set client config
testd config keyring-backend $KEYRING
testd config chain-id $CHAINID

# if $KEY exists it should be deleted
testd keys add $KEY --keyring-backend $KEYRING --algo $KEYALGO

# Allocate genesis accounts (cosmos formatted addresses)
testd add-genesis-account $KEY 100000000000000000000000000utest --keyring-backend $KEYRING

# Sign genesis transaction
testd gentx $KEY 1000000000000000000000utest --keyring-backend $KEYRING --chain-id $CHAINID

# Collect genesis tx
testd collect-gentxs

# Run this to ensure everything worked and that the genesis file is setup correctly
testd validate-genesis

# Change parameter token denominations to aacre
cat $HOME/.testd/config/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="utest"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="utest"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="utest"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["gov"]["voting_params"]["voting_period"]="5000s"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["evm"]["params"]["evm_denom"]="utest"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["mint"]["params"]["mint_denom"]="utest"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["mint"]["params"]["next_rewards_reduction_time"]="'$(($(date -u +%s) + 30))'"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq '.app_state["mint"]["params"]["minting_rewards_distribution_start_time"]="'$(($(date -u +%s) + 90))'"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# Set gas limit in genesis
cat $HOME/.testd/config/genesis.json | jq '.consensus_params["block"]["max_gas"]="10000000"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# Set claims start time
node_address=$(testd keys list | grep  "address: " | cut -c12-)
current_date=$(date -u +"%Y-%m-%dT%TZ")
cat $HOME/.testd/config/genesis.json | jq -r --arg current_date "$current_date" '.app_state["claims"]["params"]["airdrop_start_time"]=$current_date' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# Set claims records for validator account
amount_to_claim=10000
cat $HOME/.testd/config/genesis.json | jq -r --arg node_address "$node_address" --arg amount_to_claim "$amount_to_claim" '.app_state["claims"]["claims_records"]=[{"initial_claimable_amount":$amount_to_claim, "actions_completed":[false, false, false, false],"address":$node_address}]' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# Set claims decay
cat $HOME/.testd/config/genesis.json | jq -r --arg current_date "$current_date" '.app_state["claims"]["params"]["duration_of_decay"]="1000000s"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json
cat $HOME/.testd/config/genesis.json | jq -r --arg current_date "$current_date" '.app_state["claims"]["params"]["duration_until_decay"]="100000s"' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# Claim module account:
# 0xA61808Fe40fEb8B3433778BBC2ecECCAA47c8c47 || acre15cvq3ljql6utxseh0zau9m8ve2j8erz8aearn8
cat $HOME/.testd/config/genesis.json | jq -r --arg amount_to_claim "$amount_to_claim" '.app_state["bank"]["balances"] += [{"address":"acre15cvq3ljql6utxseh0zau9m8ve2j8erz8aearn8","coins":[{"denom":"utest", "amount":$amount_to_claim}]}]' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

# disable produce empty block
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' 's/create_empty_blocks = true/create_empty_blocks = false/g' $HOME/.testd/config/config.toml
  else
    sed -i 's/create_empty_blocks = true/create_empty_blocks = false/g' $HOME/.testd/config/config.toml
fi

if [[ $1 == "pending" ]]; then
  if [[ "$OSTYPE" == "darwin"* ]]; then
      sed -i '' 's/create_empty_blocks_interval = "0s"/create_empty_blocks_interval = "30s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_propose = "3s"/timeout_propose = "30s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_prevote = "1s"/timeout_prevote = "10s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_precommit = "1s"/timeout_precommit = "10s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_commit = "5s"/timeout_commit = "150s"/g' $HOME/.testd/config/config.toml
      sed -i '' 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "150s"/g' $HOME/.testd/config/config.toml
  else
      sed -i 's/create_empty_blocks_interval = "0s"/create_empty_blocks_interval = "30s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_propose = "3s"/timeout_propose = "30s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_prevote = "1s"/timeout_prevote = "10s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_precommit = "1s"/timeout_precommit = "10s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "5s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_commit = "5s"/timeout_commit = "150s"/g' $HOME/.testd/config/config.toml
      sed -i 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "150s"/g' $HOME/.testd/config/config.toml
  fi
fi

# Update total supply with claim values
validators_supply=$(cat $HOME/.testd/config/genesis.json | jq -r '.app_state["bank"]["supply"][0]["amount"]')
# Bc is required to add this big numbers
# total_supply=$(bc <<< "$amount_to_claim+$validators_supply")
total_supply=100000000000000000000010000
cat $HOME/.testd/config/genesis.json | jq -r --arg total_supply "$total_supply" '.app_state["bank"]["supply"][0]["amount"]=$total_supply' > $HOME/.testd/config/tmp_genesis.json && mv $HOME/.testd/config/tmp_genesis.json $HOME/.testd/config/genesis.json

if [[ $1 == "pending" ]]; then
  echo "pending mode is on, please wait for the first block committed."
fi

# Start the node (remove the --pruning=nothing flag if historical queries are not needed)
testd start --pruning=nothing $TRACE --log_level $LOGLEVEL --minimum-gas-prices=0.0001utest --json-rpc.api eth,txpool,personal,net,debug,web3
