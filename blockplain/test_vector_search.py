"""
test_vector_search.py - Test script for the Cross-Chain Intelligence Engine vector search
Simulates blockchain transactions and demonstrates vector search capabilities
"""

import numpy as np
import time
import json
import random
from datetime import datetime, timedelta
from vector_search import VectorSearchEngine, extract_transaction_features
import logging
import os

# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger('test_vector_search')

# Create directories for vector data if they don't exist
os.makedirs("./blockchain_vectors", exist_ok=True)
os.makedirs("./blockchain_metadata", exist_ok=True)

def generate_mock_transaction(chain="ethereum"):
    """Generate a mock blockchain transaction"""
    chains = {
        "ethereum": {
            "value_range": (0.001, 10.0),
            "gas_range": (21000, 500000),
            "gas_price_range": (10, 200),
            "address_prefix": "0x",
            "address_length": 40
        },
        "bitcoin": {
            "value_range": (0.0001, 5.0),
            "gas_range": (0, 0),  # Bitcoin doesn't use gas
            "gas_price_range": (0, 0),
            "address_prefix": "bc1",
            "address_length": 32
        },
        "solana": {
            "value_range": (0.01, 100.0),
            "gas_range": (1000, 5000),
            "gas_price_range": (1, 10),
            "address_prefix": "",
            "address_length": 44
        }
    }
    
    chain_config = chains.get(chain, chains["ethereum"])
    
    # Generate random sender and receiver addresses
    def random_address(prefix, length):
        addr = ''.join(random.choice('0123456789abcdef') for _ in range(length))
        return f"{prefix}{addr}"
    
    sender = random_address(chain_config["address_prefix"], chain_config["address_length"])
    receiver = random_address(chain_config["address_prefix"], chain_config["address_length"])
    
    # Random transaction values
    value = random.uniform(*chain_config["value_range"])
    
    # Generate a random timestamp within the last week
    now = datetime.now()
    random_time = now - timedelta(
        days=random.randint(0, 7),
        hours=random.randint(0, 23),
        minutes=random.randint(0, 59)
    )
    
    # Basic transaction object
    transaction = {
        "hash": f"{chain_config['address_prefix']}{random.randint(0, 2**64):x}",
        "chain": chain,
        "sender": sender,
        "receiver": receiver,
        "value": value,
        "timestamp": random_time.isoformat(),
        "block": random.randint(1000000, 2000000)
    }
    
    # Add chain-specific fields
    if chain == "ethereum":
        transaction["gas"] = random.randint(*chain_config["gas_range"])
        transaction["gasPrice"] = random.randint(*chain_config["gas_price_range"])
        transaction["nonce"] = random.randint(0, 1000)
    elif chain == "bitcoin":
        transaction["confirmations"] = random.randint(0, 6)
        transaction["fee"] = random.uniform(0.0001, 0.001)
    elif chain == "solana":
        transaction["fee"] = random.uniform(0.000001, 0.00001)
        transaction["slot"] = random.randint(100000000, 200000000)
    
    return transaction

def mock_embedding_model(features, dimension=384):
    """
    Simulate an embedding model that converts transaction features to vectors
    In a real system, this would use a trained model
    """
    # Start with features
    feature_array = np.array(features, dtype=np.float32)
    
    # Pad or truncate to ensure consistent length
    if len(feature_array) < 5:
        feature_array = np.pad(feature_array, (0, 5 - len(feature_array)), 'constant')
    elif len(feature_array) > 5:
        feature_array = feature_array[:5]
    
    # Expand to desired dimension with random values, but seeded by the features
    # This ensures similar transactions get similar embeddings
    random_gen = np.random.RandomState(int(abs(hash(str(feature_array.tolist()))) % (2**32)))
    embedding = random_gen.randn(dimension).astype(np.float32)
    
    # Inject the actual features into the first positions
    embedding[:len(feature_array)] = feature_array
    
    # Normalize the vector
    embedding = embedding / np.linalg.norm(embedding)
    
    return embedding

def create_anomalous_transaction(base_transaction, anomaly_type="value_spike"):
    """Create an anomalous transaction based on a normal one"""
    anomalous_tx = base_transaction.copy()
    
    if anomaly_type == "value_spike":
        # Drastically increase the value
        anomalous_tx["value"] = base_transaction["value"] * random.uniform(50, 100)
    elif anomaly_type == "gas_spike":
        # Only for Ethereum-like chains
        if "gas" in base_transaction:
            anomalous_tx["gas"] = base_transaction["gas"] * random.uniform(10, 20)
    elif anomaly_type == "unusual_time":
        # Set to unusual hour (3-4 AM)
        dt = datetime.fromisoformat(base_transaction["timestamp"].replace('Z', '+00:00'))
        unusual_dt = dt.replace(hour=random.randint(3, 4), minute=random.randint(0, 59))
        anomalous_tx["timestamp"] = unusual_dt.isoformat()
    elif anomaly_type == "circular":
        # Make sender and receiver the same
        anomalous_tx["receiver"] = anomalous_tx["sender"]
    
    # Mark as anomalous for testing purposes
    anomalous_tx["is_anomalous"] = True
    anomalous_tx["anomaly_type"] = anomaly_type
    
    return anomalous_tx

def main():
    print("="*80)
    print("CROSS-CHAIN INTELLIGENCE ENGINE - VECTOR SEARCH TEST")
    print("="*80)
    print("\nInitializing vector search engine...")
    
    # Create the vector search engine
    engine = VectorSearchEngine(
        dimension=384,
        index_type="IVFFlat",
        metric="cosine",
        nlist=100,
        use_gpu=False,  # Set to True if GPU is available
        storage_path="./blockchain_vectors",
        metadata_storage_path="./blockchain_metadata"
    )
    
    # List of chains to simulate
    chains = ["ethereum", "bitcoin", "solana"]
    
    # Number of normal transactions to generate per chain
    num_normal_tx = 1000
    
    # Number of anomalous transactions to generate per chain
    num_anomalous_tx = 20
    
    # Dictionary to store generated transactions
    transactions = {chain: [] for chain in chains}
    anomalies = {chain: [] for chain in chains}
    
    print("\nGenerating mock blockchain transactions...")
    for chain in chains:
        print(f"\nProcessing {chain} chain...")
        
        # Generate normal transactions
        print(f"- Generating {num_normal_tx} normal transactions")
        for _ in range(num_normal_tx):
            tx = generate_mock_transaction(chain)
            transactions[chain].append(tx)
        
        # Generate anomalous transactions
        print(f"- Generating {num_anomalous_tx} anomalous transactions")
        anomaly_types = ["value_spike", "unusual_time", "circular"]
        if chain == "ethereum":
            anomaly_types.append("gas_spike")
            
        for _ in range(num_anomalous_tx):
            # Pick a random transaction to base the anomaly on
            base_tx = random.choice(transactions[chain])
            anomaly_type = random.choice(anomaly_types)
            anomalous_tx = create_anomalous_transaction(base_tx, anomaly_type)
            anomalies[chain].append(anomalous_tx)
    
    print("\nIndexing transactions in vector database...")
    # Process and index normal transactions
    for chain in chains:
        print(f"- Indexing {len(transactions[chain])} normal {chain} transactions")
        
        batch_vectors = []
        batch_metadata = []
        
        for tx in transactions[chain]:
            # Extract features from transaction
            features = extract_transaction_features(tx)
            
            # Convert features to embedding
            embedding = mock_embedding_model(features)
            
            # For testing, store the embedding in the metadata
            tx_metadata = tx.copy()
            tx_metadata["vector"] = embedding.tolist()
            
            batch_vectors.append(embedding)
            batch_metadata.append(tx_metadata)
        
        # Convert to numpy array
        batch_vectors_np = np.array(batch_vectors, dtype=np.float32)
        
        # Add to vector search engine
        engine.add_vectors(batch_vectors_np, batch_metadata, chain)
    
    print("\nSearching for similar transactions...")
    # Try finding similar transactions to a random one
    for chain in chains:
        print(f"\nFinding similar transactions in {chain}:")
        random_tx = random.choice(transactions[chain])
        features = extract_transaction_features(random_tx)
        query_vector = mock_embedding_model(features)
        
        # Search for similar transactions
        ids, distances, metadata = engine.search(query_vector, k=5, context_name=chain)
        
        print(f"Query transaction: {random_tx['hash']} - Value: {random_tx['value']}")
        print(f"Found {len(ids)} similar transactions:")
        for i, (id, dist, meta) in enumerate(zip(ids, distances, metadata)):
            print(f"  {i+1}. Transaction: {meta.get('hash')} - Value: {meta.get('value')}")
            print(f"     Similarity: {dist:.4f}")
    
    print("\nTesting anomaly detection...")
    # Now test with anomalous transactions
    for chain in chains:
        print(f"\nTesting anomaly detection in {chain}:")
        anomaly_count = 0
        
        for anomalous_tx in anomalies[chain]:
            features = extract_transaction_features(anomalous_tx)
            query_vector = mock_embedding_model(features)
            
            # Search for similar transactions
            ids, distances, metadata = engine.search(query_vector, k=5, context_name=chain)
            
            # Check if this is flagged as potentially anomalous
            # For this simple test, we'll say it's anomalous if the cosine similarity
            # to the most similar transaction is below a threshold
            if len(distances) > 0 and distances[0] < 0.85:
                anomaly_count += 1
                print(f"Detected anomaly: {anomalous_tx['hash']} - Type: {anomalous_tx.get('anomaly_type')}")
                print(f"  Value: {anomalous_tx['value']}")
                print(f"  Best match similarity: {distances[0]:.4f}")
        
        print(f"Detected {anomaly_count} out of {len(anomalies[chain])} anomalies in {chain}")
    
    print("\nTesting cross-chain search...")
    # Test searching across all chains
    random_chain = random.choice(chains)
    random_tx = random.choice(transactions[random_chain])
    features = extract_transaction_features(random_tx)
    query_vector = mock_embedding_model(features)
    
    print(f"Searching for transactions similar to {random_tx['hash']} from {random_chain} across all chains:")
    results = engine.multi_index_search(query_vector, k=3, contexts=chains)
    
    for chain, (ids, distances, metadata) in results.items():
        print(f"\nResults from {chain}:")
        for i, (id, dist, meta) in enumerate(zip(ids, distances, metadata)):
            print(f"  {i+1}. Transaction: {meta.get('hash')} - Value: {meta.get('value')}")
            print(f"     Similarity: {dist:.4f}")
    
    print("\nSaving vector database...")
    engine.save_all()
    
    print("\nVector search test completed!")
    print("="*80)

if __name__ == "__main__":
    main()