flowchart TD
    subgraph "Blockplain Framework (Go)"
        BC[Blockchain Core]
        MP[Multiple Planes]
        CS[Consensus System]
        BR[Bridge Module]
        VM[Validator Manager]
    end
    
    subgraph "Integration Layer"
        PS[Pub/Sub System]
        WS[WebSocket Server]
        CM[Connection Manager]
    end
    
    subgraph "Pathway Layer (Python)"
        DS[Data Streams]
        EM[Embeddings Module]
        AD[Anomaly Detection]
        LLM[LLM Integration]
        VS[Vector Search - FAISS]
    end
    
    subgraph "Response Layer"
        FG[Fraud Detection]
        CC[Compliance Checks]
        MEV[MEV Pattern Tracing]
        RR[Rerouting Recommendations]
        SL[Slashing Logic]
    end
    
    subgraph "Frontend (React+TypeScript)"
        UI[User Interface]
        VZ[Visualization - Three.js]
        TS[Transaction Stream View]
        FE[Flagged Events Dashboard]
        AI[AI Decision Explorer]
        AT[Audit Trail]
    end
    
    BC --> MP
    MP --> CS
    MP --> BR
    CS --> VM
    
    BC --> PS
    BR --> PS
    CS --> PS
    VM --> PS
    
    PS --> CM
    WS --> CM
    CM --> DS
    
    DS --> EM
    DS --> AD
    EM --> VS
    AD --> VS
    VS --> LLM
    
    LLM --> FG
    LLM --> CC
    LLM --> MEV
    LLM --> RR
    LLM --> SL
    
    FG --> WS
    CC --> WS
    MEV --> WS
    RR --> WS
    SL --> WS
    
    WS --> UI
    UI --> VZ
    UI --> TS
    UI --> FE
    UI --> AI
    UI --> AT
