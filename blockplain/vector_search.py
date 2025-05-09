"""
vector_search.py - Vector search module for Cross-Chain Intelligence Engine
Provides FAISS-based vector storage and similarity search for blockchain transaction patterns
"""

import os
import json
import time
import numpy as np
import faiss
import logging
from typing import Dict, List, Tuple, Optional, Union, Any
import pickle
from datetime import datetime

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger('vector_search')

class VectorSearchEngine:
    """FAISS-based vector search engine for blockchain transaction pattern matching"""
    
    def __init__(
        self, 
        dimension: int = 384,
        index_type: str = "IVFFlat",
        metric: str = "l2",
        nlist: int = 100,
        use_gpu: bool = False,
        storage_path: str = "./vector_data",
        metadata_storage_path: str = "./vector_metadata"
    ):
        """
        Initialize the vector search engine
        
        Args:
            dimension: Vector dimension (default: 384 for typical embeddings)
            index_type: FAISS index type (IVFFlat, HNSW, Flat)
            metric: Distance metric (l2 or cosine)
            nlist: Number of clusters for IVF indices
            use_gpu: Whether to use GPU acceleration
            storage_path: Path to store the FAISS indices
            metadata_storage_path: Path to store associated metadata
        """
        self.dimension = dimension
        self.index_type = index_type
        self.metric = metric
        self.nlist = nlist
        self.use_gpu = use_gpu
        self.storage_path = storage_path
        self.metadata_storage_path = metadata_storage_path
        
        # Create storage directories if they don't exist
        os.makedirs(storage_path, exist_ok=True)
        os.makedirs(metadata_storage_path, exist_ok=True)
        
        # Dictionary to store indices for different chains/contexts
        self.indices = {}
        
        # Dictionary to store metadata associated with vectors
        self.metadata = {}
        
        # Initialize standard index for general queries
        self._initialize_index("general")
        
        logger.info(f"Vector search engine initialized with {dimension}d vectors using {index_type} index")
    
    def _initialize_index(self, context_name: str) -> None:
        """
        Initialize a FAISS index for a specific context (chain or analysis type)
        
        Args:
            context_name: Name of the context/chain for this index
        """
        # Define distance metric
        if self.metric == "cosine":
            # For cosine similarity, we need to normalize vectors
            metric_param = faiss.METRIC_INNER_PRODUCT
        else:  # Default to L2
            metric_param = faiss.METRIC_L2

        # Create appropriate index based on type
        if self.index_type == "Flat":
            # Flat index (exact search, but slower)
            index = faiss.IndexFlat(self.dimension, metric_param)
        
        elif self.index_type == "HNSW":
            # HNSW index (approximate search, good balance of speed and accuracy)
            index = faiss.IndexHNSWFlat(self.dimension, 32, metric_param)
            # M=32 for good accuracy, you can tune this parameter
        
        elif self.index_type == "IVFFlat":
            # IVF with flat quantizer (good for medium-sized datasets)
            quantizer = faiss.IndexFlat(self.dimension, metric_param)
            index = faiss.IndexIVFFlat(quantizer, self.dimension, self.nlist, metric_param)
            # Need training data before use
        
        else:
            # Default to IVFFlat
            quantizer = faiss.IndexFlat(self.dimension, metric_param)
            index = faiss.IndexIVFFlat(quantizer, self.dimension, self.nlist, metric_param)
        
        # Move to GPU if requested and available
        if self.use_gpu:
            try:
                res = faiss.StandardGpuResources()
                index = faiss.index_cpu_to_gpu(res, 0, index)
                logger.info(f"Successfully moved index '{context_name}' to GPU")
            except Exception as e:
                logger.warning(f"Failed to use GPU for FAISS: {str(e)}. Falling back to CPU.")
        
        # Store the index
        self.indices[context_name] = index
        
        # Initialize empty metadata store for this context
        self.metadata[context_name] = []
        
        # Try to load existing index and metadata if they exist
        self._load_index(context_name)
        
    def _requires_training(self, index):
        """Check if the index requires training before adding vectors"""
        return isinstance(index, faiss.IndexIVFFlat) or "IVF" in self.index_type
    
    def train_index(self, context_name: str, training_vectors: np.ndarray) -> None:
        """
        Train an IVF-type index with sample vectors
        
        Args:
            context_name: Name of the context/chain index to train
            training_vectors: Sample vectors for training (numpy ndarray)
        """
        if context_name not in self.indices:
            self._initialize_index(context_name)
            
        index = self.indices[context_name]
        
        if self._requires_training(index) and not index.is_trained:
            if training_vectors.shape[1] != self.dimension:
                raise ValueError(f"Training vectors dimension ({training_vectors.shape[1]}) " 
                                f"doesn't match index dimension ({self.dimension})")
                
            logger.info(f"Training index '{context_name}' with {len(training_vectors)} vectors")
            index.train(training_vectors)
            logger.info(f"Index '{context_name}' training completed")
    
    def add_vectors(
        self, 
        vectors: np.ndarray, 
        metadata_list: List[Dict[str, Any]], 
        context_name: str = "general"
    ) -> List[int]:
        """
        Add vectors and their metadata to the index
        
        Args:
            vectors: Numpy array of vectors to add [n, dimension]
            metadata_list: List of metadata dictionaries corresponding to vectors
            context_name: Name of the context/chain index to add to
            
        Returns:
            List of vector IDs assigned
        """
        if context_name not in self.indices:
            self._initialize_index(context_name)
        
        index = self.indices[context_name]
        
        # Check vector dimensions
        if vectors.shape[1] != self.dimension:
            raise ValueError(f"Input vector dimension ({vectors.shape[1]}) "
                           f"doesn't match index dimension ({self.dimension})")
        
        # Check if index needs training
        if self._requires_training(index) and not index.is_trained:
            # Use these vectors as training data if no other is provided
            self.train_index(context_name, vectors)
        
        # Normalize for cosine similarity if needed
        if self.metric == "cosine":
            faiss.normalize_L2(vectors)
            
        # Get current index size to assign new IDs
        current_size = self.get_index_size(context_name)
        new_ids = np.arange(current_size, current_size + len(vectors)).astype('int64')
        
        # Add vectors to index
        index.add_with_ids(vectors, new_ids)
        
        # Add metadata with timestamp
        for i, meta in enumerate(metadata_list):
            # Add timestamp and ID to metadata
            meta_with_ts = meta.copy()
            meta_with_ts["_timestamp"] = datetime.now().isoformat()
            meta_with_ts["_vector_id"] = int(new_ids[i])
            
            self.metadata[context_name].append(meta_with_ts)
        
        logger.info(f"Added {len(vectors)} vectors to context '{context_name}'")
        
        # Save index and metadata periodically or if size exceeds threshold
        if len(vectors) > 100 or len(self.metadata[context_name]) % 1000 == 0:
            self._save_index(context_name)
            
        return new_ids.tolist()
    
    def search(
        self, 
        query_vector: np.ndarray, 
        k: int = 10, 
        context_name: str = "general",
        threshold: float = None
    ) -> Tuple[List[int], List[float], List[Dict[str, Any]]]:
        """
        Search for similar vectors
        
        Args:
            query_vector: Query vector [1, dimension]
            k: Number of results to return
            context_name: Context/chain to search in
            threshold: Optional similarity threshold (depends on metric)
            
        Returns:
            Tuple of (ids, distances, metadata)
        """
        if context_name not in self.indices:
            logger.warning(f"Index '{context_name}' not found. Using 'general' index instead.")
            context_name = "general"
            
            if context_name not in self.indices:
                logger.error("No indices available for search")
                return [], [], []
        
        index = self.indices[context_name]
        
        # Reshape query if needed
        if len(query_vector.shape) == 1:
            query_vector = query_vector.reshape(1, -1)
            
        # Check dimensions
        if query_vector.shape[1] != self.dimension:
            raise ValueError(f"Query vector dimension ({query_vector.shape[1]}) " 
                           f"doesn't match index dimension ({self.dimension})")
        
        # Normalize for cosine similarity if needed
        if self.metric == "cosine":
            faiss.normalize_L2(query_vector)
        
        # Execute search
        if self.get_index_size(context_name) == 0:
            logger.warning(f"Index '{context_name}' is empty")
            return [], [], []
            
        distances, indices = index.search(query_vector, k)
        
        # Flatten results
        result_ids = indices[0].tolist()
        result_dists = distances[0].tolist()
        
        # Filter by threshold if provided
        if threshold is not None:
            if self.metric == "cosine":
                # For cosine, higher is better (closer to 1)
                filtered_results = [(i, d) for i, d in zip(result_ids, result_dists) if d >= threshold]
            else:
                # For L2, lower is better (closer to 0)
                filtered_results = [(i, d) for i, d in zip(result_ids, result_dists) if d <= threshold]
                
            if filtered_results:
                result_ids, result_dists = zip(*filtered_results)
            else:
                return [], [], []
        
        # Gather metadata for results
        result_metadata = []
        for vector_id in result_ids:
            # Find metadata with matching ID
            for meta in self.metadata[context_name]:
                if meta.get("_vector_id") == vector_id:
                    result_metadata.append(meta)
                    break
        
        return result_ids, result_dists, result_metadata
    
    def multi_index_search(
        self, 
        query_vector: np.ndarray, 
        k: int = 10,
        contexts: List[str] = None,
        threshold: float = None
    ) -> Dict[str, Tuple[List[int], List[float], List[Dict[str, Any]]]]:
        """
        Search across multiple context indices
        
        Args:
            query_vector: Query vector
            k: Number of results per context
            contexts: List of contexts to search (None = all)
            threshold: Optional similarity threshold
            
        Returns:
            Dictionary mapping context names to search results
        """
        if contexts is None:
            contexts = list(self.indices.keys())
            
        results = {}
        for ctx in contexts:
            if ctx in self.indices:
                ids, dists, metas = self.search(query_vector, k, ctx, threshold)
                results[ctx] = (ids, dists, metas)
                
        return results
    
    def get_index_size(self, context_name: str = "general") -> int:
        """Get the number of vectors in an index"""
        if context_name not in self.indices:
            return 0
        return self.indices[context_name].ntotal
    
    def _save_index(self, context_name: str) -> None:
        """Save index and metadata to disk"""
        try:
            # Get CPU version of index if on GPU
            if self.use_gpu:
                index_to_save = faiss.index_gpu_to_cpu(self.indices[context_name])
            else:
                index_to_save = self.indices[context_name]
                
            # Save FAISS index
            index_path = os.path.join(self.storage_path, f"{context_name}.faiss")
            faiss.write_index(index_to_save, index_path)
            
            # Save metadata
            metadata_path = os.path.join(self.metadata_storage_path, f"{context_name}_metadata.pkl")
            with open(metadata_path, 'wb') as f:
                pickle.dump(self.metadata[context_name], f)
                
            logger.info(f"Saved index '{context_name}' with {self.get_index_size(context_name)} vectors")
            
        except Exception as e:
            logger.error(f"Failed to save index '{context_name}': {str(e)}")
    
    def _load_index(self, context_name: str) -> bool:
        """Load index and metadata from disk if they exist"""
        index_path = os.path.join(self.storage_path, f"{context_name}.faiss")
        metadata_path = os.path.join(self.metadata_storage_path, f"{context_name}_metadata.pkl")
        
        try:
            # Check if files exist
            if os.path.exists(index_path) and os.path.exists(metadata_path):
                # Load index
                loaded_index = faiss.read_index(index_path)
                
                # Move to GPU if needed
                if self.use_gpu:
                    try:
                        res = faiss.StandardGpuResources()
                        loaded_index = faiss.index_cpu_to_gpu(res, 0, loaded_index)
                    except Exception as e:
                        logger.warning(f"Failed to move loaded index to GPU: {str(e)}")
                
                self.indices[context_name] = loaded_index
                
                # Load metadata
                with open(metadata_path, 'rb') as f:
                    self.metadata[context_name] = pickle.load(f)
                    
                logger.info(f"Loaded index '{context_name}' with {self.get_index_size(context_name)} vectors")
                return True
                
        except Exception as e:
            logger.error(f"Failed to load index '{context_name}': {str(e)}")
            # Initialize a new index as fallback
            self._initialize_index(context_name)
            
        return False
    
    def delete_vectors(self, vector_ids: List[int], context_name: str = "general") -> bool:
        """
        Delete vectors from the index (note: FAISS has limited deletion capabilities)
        For some index types, this will rebuild the index
        
        Args:
            vector_ids: List of vector IDs to delete
            context_name: Context name
            
        Returns:
            Success status
        """
        if context_name not in self.indices:
            logger.warning(f"Index '{context_name}' not found")
            return False
            
        try:
            # For index types that support it, use direct removal
            index = self.indices[context_name]
            
            # Note: This is a limitation of FAISS - not all index types support remove_ids
            # Indices like FlatIndex can support this, but for others we would need
            # to rebuild the index excluding those vectors
            if hasattr(index, 'remove_ids'):
                index.remove_ids(np.array(vector_ids, dtype='int64'))
                
                # Update metadata
                self.metadata[context_name] = [
                    meta for meta in self.metadata[context_name] 
                    if meta.get("_vector_id") not in vector_ids
                ]
                
                logger.info(f"Deleted {len(vector_ids)} vectors from '{context_name}'")
                self._save_index(context_name)
                return True
            else:
                logger.warning(f"Direct deletion not supported for index type {self.index_type}")
                return False
                
        except Exception as e:
            logger.error(f"Error deleting vectors: {str(e)}")
            return False
    
    def rebuild_index(self, context_name: str = "general") -> bool:
        """
        Rebuild an index from scratch using existing metadata
        Useful after many deletions or for compaction
        
        Args:
            context_name: Context to rebuild
            
        Returns:
            Success status
        """
        if context_name not in self.indices or context_name not in self.metadata:
            logger.warning(f"Cannot rebuild non-existent index '{context_name}'")
            return False
            
        try:
            # Extract existing vectors and metadata
            # This assumes we have vector data in metadata - adjust as needed
            vectors = []
            metadata_list = []
            
            for meta in self.metadata[context_name]:
                if "vector" in meta and meta["vector"] is not None:
                    vectors.append(meta["vector"])
                    metadata_list.append(meta)
            
            if not vectors:
                logger.warning(f"No vectors found to rebuild index '{context_name}'")
                return False
                
            # Create new index with same parameters
            old_index = self.indices[context_name]
            self._initialize_index(context_name)
            
            # Add all vectors back
            vectors_array = np.array(vectors, dtype='float32')
            self.add_vectors(vectors_array, metadata_list, context_name)
            
            logger.info(f"Successfully rebuilt index '{context_name}' with {len(vectors)} vectors")
            return True
            
        except Exception as e:
            logger.error(f"Failed to rebuild index: {str(e)}")
            return False

    def save_all(self) -> None:
        """Save all indices to disk"""
        for context_name in self.indices.keys():
            self._save_index(context_name)
    
    def load_all(self) -> None:
        """Load all indices from disk"""
        # Find all available index files
        for filename in os.listdir(self.storage_path):
            if filename.endswith('.faiss'):
                context_name = filename.replace('.faiss', '')
                self._load_index(context_name)

    def batch_search(
        self, 
        query_vectors: np.ndarray, 
        k: int = 10, 
        context_name: str = "general",
        threshold: float = None
    ) -> Tuple[np.ndarray, np.ndarray]:
        """
        Batch search for multiple query vectors at once
        
        Args:
            query_vectors: Query vectors [n, dimension]
            k: Number of results per query
            context_name: Context to search in
            threshold: Optional similarity threshold
            
        Returns:
            Tuple of (distances, indices) arrays
        """
        if context_name not in self.indices:
            logger.warning(f"Index '{context_name}' not found")
            return np.array([]), np.array([])
            
        index = self.indices[context_name]
        
        # Check dimensions
        if query_vectors.shape[1] != self.dimension:
            raise ValueError(f"Query vectors dimension ({query_vectors.shape[1]}) " 
                           f"doesn't match index dimension ({self.dimension})")
        
        # Normalize for cosine similarity if needed
        if self.metric == "cosine":
            faiss.normalize_L2(query_vectors)
            
        # Execute search
        distances, indices = index.search(query_vectors, k)
        
        # Apply threshold if provided
        if threshold is not None:
            if self.metric == "cosine":
                # For cosine, higher is better
                mask = distances >= threshold
            else:
                # For L2, lower is better
                mask = distances <= threshold
                
            # Set non-matching results to -1 index and max/min distance
            indices[~mask] = -1
            if self.metric == "cosine":
                distances[~mask] = -1.0
            else:
                distances[~mask] = float('inf')
                
        return distances, indices
    
    def __del__(self):
        """Save all indices when object is destroyed"""
        self.save_all()


# Example usage and helper functions
def create_blockchain_vector_engine(use_gpu=False):
    """Create a vector search engine with blockchain-specific settings"""
    return VectorSearchEngine(
        dimension=384,  # Adjust based on your embedding model
        index_type="IVFFlat", 
        metric="cosine",  # Cosine similarity is often better for semantic searches
        nlist=100,
        use_gpu=use_gpu,
        storage_path="./blockchain_vectors",
        metadata_storage_path="./blockchain_metadata"
    )

def extract_transaction_features(transaction_data):
    """
    Extract numerical features from blockchain transaction data
    Returns a list of features that can be converted to embeddings
    
    Args:
        transaction_data: Raw transaction data
        
    Returns:
        List of features
    """
    # This is just an example - adapt to your specific transaction format
    features = []
    
    try:
        # Example features (customize based on your data)
        if "value" in transaction_data:
            features.append(float(transaction_data["value"]))
        else:
            features.append(0.0)
            
        if "gas" in transaction_data:
            features.append(float(transaction_data["gas"]))
        else:
            features.append(0.0)
            
        if "gasPrice" in transaction_data:
            features.append(float(transaction_data["gasPrice"]))
        else:
            features.append(0.0)
            
        # Sender/receiver patterns can be embedded separately
        
        # Add timestamp features
        if "timestamp" in transaction_data:
            ts = transaction_data["timestamp"]
            if isinstance(ts, str):
                # Try to parse as ISO format
                try:
                    dt = datetime.fromisoformat(ts.replace('Z', '+00:00'))
                    features.append(dt.hour / 24.0)  # Hour of day (normalized)
                    features.append(dt.weekday() / 7.0)  # Day of week (normalized)
                except ValueError:
                    # Try timestamp as unix time
                    try:
                        dt = datetime.fromtimestamp(float(ts))
                        features.append(dt.hour / 24.0)
                        features.append(dt.weekday() / 7.0)
                    except:
                        features.extend([0.0, 0.0])
            elif isinstance(ts, (int, float)):
                dt = datetime.fromtimestamp(ts)
                features.append(dt.hour / 24.0)
                features.append(dt.weekday() / 7.0)
            else:
                features.extend([0.0, 0.0])
        else:
            features.extend([0.0, 0.0])
            
    except Exception as e:
        logger.error(f"Error extracting transaction features: {str(e)}")
        # Return zeros if extraction fails
        return [0.0] * 5
        
    return features


if __name__ == "__main__":
    # Example usage
    engine = create_blockchain_vector_engine()
    
    # Generate some random vectors for testing
    test_vectors = np.random.rand(10, 384).astype('float32')
    
    # Create sample metadata
    test_metadata = [
        {"tx_hash": f"0x{i:064x}", "chain": "ethereum", "block": 1000+i, "value": 0.1*i} 
        for i in range(10)
    ]
    
    # Add to index
    engine.add_vectors(test_vectors, test_metadata, "ethereum")
    
    # Test search
    query = np.random.rand(1, 384).astype('float32')
    ids, distances, metadata = engine.search(query, k=3, context_name="ethereum")
    
    print(f"Found {len(ids)} matches")
    for i in range(len(ids)):
        print(f"Match {i+1}: Distance {distances[i]:.4f}, TX: {metadata[i]['tx_hash']}")
    
    # Save indices
    engine.save_all()