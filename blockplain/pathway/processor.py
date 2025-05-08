import os
import json
import time
import logging
import asyncio
import pathway as pw
import numpy as np
from typing import Dict, List, Any, Optional
import websockets
from datetime import datetime

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Configuration
WEBSOCKET_URL = os.environ.get("WEBSOCKET_URL", "ws://localhost:8080/ws")
EMBEDDING_DIMENSION = 384  # Dimension for the embeddings
ANOMALY_THRESHOLD = 0.85   # Threshold for anomaly detection
LLM_API_KEY = os.environ.get("LLM_API_KEY", "")
USE_LLM = os.environ.get("USE_LLM", "true").lower() == "true"

class BlockchainProcessor:
    """
    Main class for processing blockchain data with Pathway
    """
    
    def __init__(self):
        self.connected = False
        self.subscription_ids = {}
        self.ws = None
        
    async def connect_websocket(self):
        """Connect to the blockchain's websocket server"""
        try:
            self.ws = await websockets.connect(WEBSOCKET_URL)
            self.connected = True
            logger.info(f"Connected to WebSocket at {WEBSOCKET_URL}")
            
            # Subscribe to relevant event types
            subscriptions = [
                {"type": "subscribe", "data": {"event_type": "new_transaction", "plane_id": ""}},
                {"type": "subscribe", "data": {"event_type": "new_block", "plane_id": ""}},
                {"type": "subscribe", "data": {"event_type": "bridge_operation", "plane_id": ""}},
                {"type": "subscribe", "data": {"event_type": "validator_update", "plane_id": ""}}
            ]
            
            for sub in subscriptions:
                await self.ws.send(json.dumps(sub))
                response = await self.ws.recv()
                resp_data = json.loads(response)
                if resp_data["type"] == "subscription_confirm":
                    event_type = resp_data["data"]["event_type"]
                    sub_id = resp_data["data"]["sub_id"]
                    self.subscription_ids[event_type] = sub_id
                    logger.info(f"Subscribed to {event_type} events with ID {sub_id}")
                    
        except Exception as e:
            self.connected = False
            logger.error(f"WebSocket connection error: {e}")
            
    async def receive_messages(self):
        """Continuously receive messages from the websocket"""
        if not self.connected:
            await self.connect_websocket()
            
        while self.connected:
            try:
                message = await self.ws.recv()
                event = json.loads(message)
                
                # Process the event based on its type
                if event["type"] == "new_transaction":
                    await self.handle_transaction(event)
                elif event["type"] == "new_block":
                    await self.handle_block(event)
                elif event["type"] == "bridge_operation":
                    await self.handle_bridge_operation(event)
                elif event["type"] == "validator_update":
                    await self.handle_validator_update(event)
                    
            except websockets.exceptions.ConnectionClosed:
                logger.warning("WebSocket connection closed")
                self.connected = False
                # Try to reconnect
                await asyncio.sleep(5)
                await self.connect_websocket()
            except Exception as e:
                logger.error(f"Error processing message: {e}")
                
    async def handle_transaction(self, event):
        """Process transaction events"""
        # Extract transaction data
        tx_data = event["data"]
        plane_id = event["plane_id"]
        
        # Add to Pathway processing pipeline
        self.transaction_input.emit(tx_data)
        
    async def handle_block(self, event):
        """Process block events"""
        # Extract block data
        block_data = event["data"]
        plane_id = event["plane_id"]
        
        # Add to Pathway processing pipeline
        self.block_input.emit(block_data)
        
    async def handle_bridge_operation(self, event):
        """Process bridge operation events"""
        # Extract bridge data
        bridge_data = event["data"]
        plane_id = event["plane_id"]
        
        # Add to Pathway processing pipeline
        self.bridge_input.emit(bridge_data)
        
    async def handle_validator_update(self, event):
        """Process validator update events"""
        # Extract validator data
        validator_data = event["data"]
        plane_id = event["plane_id"]
        
        # Add to Pathway processing pipeline
        self.validator_input.emit(validator_data)
                
    async def send_ai_decision(self, decision_data):
        """Send AI decisions back to the blockchain system"""
        if not self.connected:
            await self.connect_websocket()
            
        try:
            message = {
                "type": "ai_decision",
                "timestamp": int(time.time()),
                "data": decision_data
            }
            
            await self.ws.send(json.dumps(message))
            logger.info(f"Sent AI decision: {decision_data['decision_type']}")
            
        except Exception as e:
            logger.error(f"Error sending AI decision: {e}")
            
    def build_pipeline(self):
        """Build the Pathway data processing pipeline"""
        # Create input connectors
        self.transaction_input = pw.io.python.write()
        self.block_input = pw.io.python.write()
        self.bridge_input = pw.io.python.write()
        self.validator_input = pw.io.python.write()
        
        # Create tables from the inputs
        transactions = pw.table_from_pandas(self.transaction_input)
        blocks = pw.table_from_pandas(self.block_input)
        bridge_ops = pw.table_from_pandas(self.bridge_input)
        validator_updates = pw.table_from_pandas(self.validator_input)
        
        # Process transactions for fraud detection
        enriched_transactions = transactions.select(
            id=pw.this.id,
            from_address=pw.this.from,
            to_address=pw.this.to,
            amount=pw.this.amount,
            asset=pw.this.asset,
            timestamp=pw.this.timestamp,
            plane_id=pw.this.plane_id,
            is_cross_chain=pw.this.is_cross_chain,
            # Create features for anomaly detection
            features=self._extract_transaction_features,
            # Generate text description for LLM analysis
            text_description=self._generate_transaction_description
        )
        
        # Process bridge operations
        enriched_bridge_ops = bridge_ops.select(
            id=pw.this.id,
            source_plane=pw.this.source_plane_id,
            target_plane=pw.this.target_plane_id,
            amount=pw.this.amount,
            asset=pw.this.asset,
            timestamp=pw.this.timestamp,
            # Create features for anomaly detection
            features=self._extract_bridge_features,
            # Generate text description for LLM analysis
            text_description=self._generate_bridge_description
        )
        
        # Process validator updates
        enriched_validator_updates = validator_updates.select(
            validator_id=pw.this.validator_id,
            plane_id=pw.this.plane_id,
            action=pw.this.action,
            timestamp=pw.this.timestamp,
            # Create features for anomaly detection
            features=self._extract_validator_features,
            # Generate text description for LLM analysis
            text_description=self._generate_validator_description
        )
        
        # Perform anomaly detection on transactions
        transaction_anomalies = enriched_transactions.select(
            id=pw.this.id,
            from_address=pw.this.from_address,
            to_address=pw.this.to_address,
            amount=pw.this.amount,
            asset=pw.this.asset,
            plane_id=pw.this.plane_id,
            timestamp=pw.this.timestamp,
            anomaly_score=self._compute_anomaly_score(pw.this.features),
            is_anomaly=self._compute_anomaly_score(pw.this.features) > ANOMALY_THRESHOLD,
            text_description=pw.this.text_description
        ).filter(pw.this.is_anomaly == True)
        
        # Perform anomaly detection on bridge operations
        bridge_anomalies = enriched_bridge_ops.select(
            id=pw.this.id,
            source_plane=pw.this.source_plane,
            target_plane=pw.this.target_plane,
            amount=pw.this.amount,
            asset=pw.this.asset,
            timestamp=pw.this.timestamp,
            anomaly_score=self._compute_anomaly_score(pw.this.features),
            is_anomaly=self._compute_anomaly_score(pw.this.features) > ANOMALY_THRESHOLD,
            text_description=pw.this.text_description
        ).filter(pw.this.is_anomaly == True)
        
        # Process anomalies with LLM if enabled
        if USE_LLM:
            # Apply LLM to transaction anomalies
            llm_transaction_insights = transaction_anomalies.select(
                id=pw.this.id,
                plane_id=pw.this.plane_id,
                timestamp=pw.this.timestamp,
                anomaly_score=pw.this.anomaly_score,
                llm_analysis=self._apply_llm_analysis(pw.this.text_description),
                decision=self._generate_fraud_decision(
                    pw.this.anomaly_score, 
                    self._apply_llm_analysis(pw.this.text_description)
                )
            )
            
            # Apply LLM to bridge anomalies
            llm_bridge_insights = bridge_anomalies.select(
                id=pw.this.id,
                source_plane=pw.this.source_plane,
                target_plane=pw.this.target_plane,
                timestamp=pw.this.timestamp,
                anomaly_score=pw.this.anomaly_score,
                llm_analysis=self._apply_llm_analysis(pw.this.text_description),
                decision=self._generate_bridge_decision(
                    pw.this.anomaly_score, 
                    self._apply_llm_analysis(pw.this.text_description)
                )
            )
            
            # Output connectors for decisions
            llm_transaction_insights.output(
                pw.io.python.callback(self._handle_transaction_decision)
            )
            
            llm_bridge_insights.output(
                pw.io.python.callback(self._handle_bridge_decision)
            )
        else:
            # Simple rule-based decisions without LLM
            transaction_decisions = transaction_anomalies.select(
                id=pw.this.id,
                plane_id=pw.this.plane_id,
                timestamp=pw.this.timestamp,
                anomaly_score=pw.this.anomaly_score,
                decision=self._generate_simple_fraud_decision(pw.this.anomaly_score)
            )
            
            bridge_decisions = bridge_anomalies.select(
                id=pw.this.id,
                source_plane=pw.this.source_plane,
                target_plane=pw.this.target_plane,
                timestamp=pw.this.timestamp,
                anomaly_score=pw.this.anomaly_score,
                decision=self._generate_simple_bridge_decision(pw.this.anomaly_score)
            )
            
            # Output connectors for decisions
            transaction_decisions.output(
                pw.io.python.callback(self._handle_transaction_decision)
            )
            
            bridge_decisions.output(
                pw.io.python.callback(self._handle_bridge_decision)
            )
            
    @staticmethod
    def _extract_transaction_features(tx):
        """Extract features from a transaction for anomaly detection"""
        # This is a simplified version - in a real system, you would extract more
        # meaningful features based on transaction history, network analysis, etc.
        features = [
            float(tx.get("amount", 0)),
            float(tx.get("timestamp", 0)),
            hash(tx.get("from", "")) % 10000 / 10000.0,  # Simple hash normalization
            hash(tx.get("to", "")) % 10000 / 10000.0,
            1.0 if tx.get("is_cross_chain", False) else 0.0,
        ]
        return features
        
    @staticmethod
    def _extract_bridge_features(bridge_op):
        """Extract features from a bridge operation for anomaly detection"""
        features = [
            float(bridge_op.get("amount", 0)),
            float(bridge_op.get("timestamp", 0)),
            hash(bridge_op.get("source_plane_id", "")) % 10000 / 10000.0,
            hash(bridge_op.get("target_plane_id", "")) % 10000 / 10000.0,
        ]
        return features
        
    @staticmethod
    def _extract_validator_features(validator_update):
        """Extract features from a validator update for anomaly detection"""
        features = [
            float(validator_update.get("timestamp", 0)),
            hash(validator_update.get("validator_id", "")) % 10000 / 10000.0,
            hash(validator_update.get("action", "")) % 10000 / 10000.0,
        ]
        return features
        
    @staticmethod
    def _generate_transaction_description(tx):
        """Generate a text description of a transaction for LLM analysis"""
        timestamp_str = datetime.fromtimestamp(
            int(tx.get("timestamp", time.time()))
        ).strftime('%Y-%m-%d %H:%M:%S')
        
        return (
            f"Transaction ID {tx.get('id', 'unknown')} from {tx.get('from', 'unknown')} "
            f"to {tx.get('to', 'unknown')} for {tx.get('amount', 0)} {tx.get('asset', 'unknown')} "
            f"on plane {tx.get('plane_id', 'unknown')} at {timestamp_str}. "
            f"This transaction {'is' if tx.get('is_cross_chain', False) else 'is not'} cross-chain."
        )
        
    @staticmethod
    def _generate_bridge_description(bridge_op):
        """Generate a text description of a bridge operation for LLM analysis"""
        timestamp_str = datetime.fromtimestamp(
            int(bridge_op.get("timestamp", time.time()))
        ).strftime('%Y-%m-%d %H:%M:%S')
        
        return (
            f"Bridge operation ID {bridge_op.get('id', 'unknown')} from plane "
            f"{bridge_op.get('source_plane_id', 'unknown')} to plane "
            f"{bridge_op.get('target_plane_id', 'unknown')} for "
            f"{bridge_op.get('amount', 0)} {bridge_op.get('asset', 'unknown')} at {timestamp_str}."
        )
        
    @staticmethod
    def _generate_validator_description(validator_update):
        """Generate a text description of a validator update for LLM analysis"""
        timestamp_str = datetime.fromtimestamp(
            int(validator_update.get("timestamp", time.time()))
        ).strftime('%Y-%m-%d %H:%M:%S')
        
        return (
            f"Validator {validator_update.get('validator_id', 'unknown')} on plane "
            f"{validator_update.get('plane_id', 'unknown')} performed action "
            f"{validator_update.get('action', 'unknown')} at {timestamp_str}."
        )
        
    @staticmethod
    def _compute_anomaly_score(features):
        """
        Compute an anomaly score for the given features
        Higher values indicate more anomalous behavior
        """
        # This is a simplified version - in a real system, you would use
        # more sophisticated anomaly detection algorithms
        
        # Normalize features (assuming they're already somewhat normalized)
        norm_features = np.array(features)
        
        # Simple anomaly score based on distance from origin
        # In a real system, this would be replaced with a proper anomaly detection model
        # such as Isolation Forest, One-class SVM, etc.
        score = min(np.linalg.norm(norm_features) / np.sqrt(len(features)), 1.0)
        
        return float(score)
        
    @staticmethod
    def _apply_llm_analysis(text_description):
        """
        Apply LLM analysis to the text description
        Returns LLM's assessment of the situation
        """
        # In a real system, this would make an API call to an LLM
        # For this implementation, we'll simulate an LLM response
        
        if "large amount" in text_description or "unknown" in text_description:
            return "This transaction appears suspicious due to the unusual amount or unknown participants."
        elif "cross-chain" in text_description:
            return "Cross-chain transactions require additional scrutiny, but this one appears normal."
        else:
            return "This transaction appears normal based on the available information."
            
    @staticmethod
    def _generate_fraud_decision(anomaly_score, llm_analysis):
        """Generate a decision based on anomaly score and LLM analysis"""
        if anomaly_score > 0.95:
            return "block"  # High confidence fraud - block transaction
        elif anomaly_score > 0.8 and "suspicious" in llm_analysis:
            return "flag"   # Suspicious transaction - flag for review
        else:
            return "monitor"  # Lower confidence - just monitor
            
    @staticmethod
    def _generate_bridge_decision(anomaly_score, llm_analysis):
        """Generate a decision for bridge operations"""
        if anomaly_score > 0.9:
            return "pause"  # High anomaly - pause bridge
        elif anomaly_score > 0.8:
            return "slow"   # Significant anomaly - slow down bridge
        else:
            return "monitor"  # Lower confidence - just monitor
            
    @staticmethod
    def _generate_simple_fraud_decision(anomaly_score):
        """Generate a simple rule-based decision without LLM"""
        if anomaly_score > 0.95:
            return "block"
        elif anomaly_score > 0.8:
            return "flag"
        else:
            return "monitor"
            
    @staticmethod
    def _generate_simple_bridge_decision(anomaly_score):
        """Generate a simple rule-based decision for bridges without LLM"""
        if anomaly_score > 0.9:
            return "pause"
        elif anomaly_score > 0.8:
            return "slow"
        else:
            return "monitor"
            
    async def _handle_transaction_decision(self, decision_data):
        """Handle a transaction decision by sending it back to the blockchain"""
        ai_decision = {
            "decision_type": "transaction",
            "transaction_id": decision_data["id"],
            "plane_id": decision_data["plane_id"],
            "timestamp": decision_data["timestamp"],
            "anomaly_score": float(decision_data["anomaly_score"]),
            "action": decision_data["decision"],
        }
        
        if "llm_analysis" in decision_data:
            ai_decision["reasoning"] = decision_data["llm_analysis"]
            
        await self.send_ai_decision(ai_decision)
        
    async def _handle_bridge_decision(self, decision_data):
        """Handle a bridge decision by sending it back to the blockchain"""
        ai_decision = {
            "decision_type": "bridge",
            "bridge_op_id": decision_data["id"],
            "source_plane": decision_data["source_plane"],
            "target_plane": decision_data["target_plane"],
            "timestamp": decision_data["timestamp"],
            "anomaly_score": float(decision_data["anomaly_score"]),
            "action": decision_data["decision"],
        }
        
        if "llm_analysis" in decision_data:
            ai_decision["reasoning"] = decision_data["llm_analysis"]
            
        await self.send_ai_decision(ai_decision)
            
    async def run(self):
        """Run the entire pipeline"""
        # Build and start the Pathway pipeline
        self.build_pipeline()
        pw.run()
        
        # Start receiving messages
        await self.receive_messages()

# Main entry point
async def main():
    processor = BlockchainProcessor()
    await processor.run()

if __name__ == "__main__":
    asyncio.run(main())