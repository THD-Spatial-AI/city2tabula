# City2TABULA Workflow Diagrams - Mermaid Code

## 1. Main Workflow Diagram

```mermaid
flowchart TD
    A[START] --> B[Initialize<br/>• Load configuration<br/>• Connect to database]
    B --> C{Check Flags}

    C -->|--create_db| D[Create Database<br/>• Create CityDB schemas<br/>• Create training schemas<br/>• Import supplementary data]
    C -->|--reset_db| E[Reset Complete<br/>• Drop all schemas<br/>• Recreate CityDB<br/>• Recreate training]
    C -->|--reset_City2TABULA| F[Reset Partial<br/>• Drop City2TABULA schema<br/>• Drop tabula schema<br/>• Recreate schemas]
    C -->|--extract_features| G[Feature Extraction<br/>• Get building IDs<br/>• Create batches<br/>• Run parallel processing]

    D --> H[Log Runtime<br/>END]
    E --> H
    F --> H
    G --> H

    style A fill:#e74c3c,stroke:#c0392b,color:#fff
    style H fill:#e74c3c,stroke:#c0392b,color:#fff
    style B fill:#2ecc71,stroke:#27ae60,color:#fff
    style C fill:#3498db,stroke:#2980b9,color:#fff
    style D fill:#2ecc71,stroke:#27ae60,color:#fff
    style E fill:#2ecc71,stroke:#27ae60,color:#fff
    style F fill:#2ecc71,stroke:#27ae60,color:#fff
    style G fill:#2ecc71,stroke:#27ae60,color:#fff
```

## 2. Complete Pipeline Workflow

```mermaid
flowchart LR
    A[Phase 1: Setup<br/>--create_db<br/>• Create CityDB schemas<br/>• Create training schemas<br/>• Import TABULA data]

    B[External Step<br/>CityDB Tools<br/>Import CityGML files<br/>into LOD2/LOD3 schemas<br/>using CityDB Importer]

    C[Phase 2: Processing<br/>--extract_features<br/>• Get building IDs<br/>• Parallel batch processing<br/>• Extract ML features]

    D[Output<br/>Training Tables:<br/>• Building features<br/>• Surface analysis<br/>• ML-ready dataset]

    E[Reset Complete<br/>--reset_db<br/>Drops everything]

    F[Reset Partial<br/>--reset_City2TABULA<br/>Preserves CityDB data]

    A -->|one-time| B
    B -->|manual| C
    C -->|automated| D

    E -.->|development cycle| A
    F -.->|development cycle| C

    style A fill:#3498db,stroke:#2980b9,color:#fff
    style B fill:#8e44ad,stroke:#7d3c98,color:#fff
    style C fill:#27ae60,stroke:#229954,color:#fff
    style D fill:#27ae60,stroke:#229954,color:#fff
    style E fill:#e67e22,stroke:#d35400,color:#fff
    style F fill:#e67e22,stroke:#d35400,color:#fff
```

## 3. Feature Extraction Pipeline

```mermaid
flowchart TD
    A[--extract_features] --> B[Get Building IDs<br/>LOD2: GetBuildingIDsFromCityDB<br/>LOD3: GetBuildingIDsFromCityDB]

    B --> C[Create Batches<br/>Batch Size: config.Batch.Size<br/>LOD2 + LOD3 batches]

    C --> D[Build Pipeline Queue<br/>BuildFeatureExtractionQueue]

    D --> E[Pipeline Channel<br/>Enqueue all pipelines]

    E --> F[Parallel Worker Pool<br/>Workers: config.Batch.Threads goroutines<br/>Each worker processes pipeline batches concurrently]

    F --> G[Worker 1<br/>Process Pipeline 1<br/>Batch: Buildings 1-1000<br/>Execute 8 SQL jobs]

    F --> H[Worker 2<br/>Process Pipeline 2<br/>Batch: Buildings 1001-2000<br/>Execute 8 SQL jobs]

    F --> I[Worker N<br/>Process Pipeline N<br/>Batch: Buildings N...<br/>Execute 8 SQL jobs]

    J[Pipeline Jobs<br/>01_get_child_feat.sql<br/>02_dump_child_feat_geom.sql<br/>03_calc_child_feat_attr.sql<br/>04_calc_bld_feat.sql<br/>06_calc_volume.sql<br/>07_calc_storeys.sql<br/>08_calc_attached_neighbours.sql<br/>09_label_building_features.sql<br/>Sequential execution per batch]

    J --> G
    J --> H
    J --> I

    G --> K[Feature Extraction Complete<br/>Training tables populated with building features<br/>Ready for machine learning model training]
    H --> K
    I --> K

    K --> L[Complete]

    style A fill:#e74c3c,stroke:#c0392b,color:#fff
    style L fill:#e74c3c,stroke:#c0392b,color:#fff
    style B fill:#2ecc71,stroke:#27ae60,color:#fff
    style C fill:#2ecc71,stroke:#27ae60,color:#fff
    style D fill:#2ecc71,stroke:#27ae60,color:#fff
    style E fill:#f39c12,stroke:#e67e22,color:#fff
    style F fill:#f39c12,stroke:#e67e22,color:#fff
    style G fill:#f39c12,stroke:#e67e22,color:#fff
    style H fill:#f39c12,stroke:#e67e22,color:#fff
    style I fill:#f39c12,stroke:#e67e22,color:#fff
    style J fill:#9b59b6,stroke:#8e44ad,color:#fff
    style K fill:#2ecc71,stroke:#27ae60,color:#fff
```

## 4. Parallel Processing Architecture

```mermaid
flowchart TD
    A[LOD2 Buildings<br/>GetBuildingIDsFromCityDB<br/>lod2.building] --> C[Batch Creation<br/>CreateBatches<br/>config.Batch.Size]

    B[LOD3 Buildings<br/>GetBuildingIDsFromCityDB<br/>lod3.building] --> C

    C --> D[Pipeline Queue<br/>BuildFeatureExtractionQueue<br/>LOD2 + LOD3 pipelines]

    D --> E[Pipeline Channel<br/>chan *process.Pipeline<br/>buffered channel]

    E --> F[Worker Pool<br/>config.Batch.Threads goroutines]

    F --> G[Worker 1<br/>goroutine<br/>process.NewWorker 1<br/>Pipeline processing]

    F --> H[Worker 2<br/>goroutine<br/>process.NewWorker 2<br/>Pipeline processing]

    F --> I[Worker 3<br/>goroutine<br/>process.NewWorker 3<br/>Pipeline processing]

    F --> J[Worker N<br/>goroutine<br/>process.NewWorker N<br/>Pipeline processing]

    F --> K[sync.WaitGroup<br/>Coordination<br/>wg.Wait<br/>Wait for completion]

    L[PostgreSQL Connection Pool<br/>Shared database connections<br/>pgxpool.Pool]

    G --> L
    H --> L
    I --> L
    J --> L

    G --> M[Training Schema<br/>Feature tables populated<br/>training.*]
    H --> M
    I --> M
    J --> M

    style A fill:#3498db,stroke:#2980b9,color:#fff
    style B fill:#3498db,stroke:#2980b9,color:#fff
    style C fill:#2ecc71,stroke:#27ae60,color:#fff
    style D fill:#2ecc71,stroke:#27ae60,color:#fff
    style E fill:#9b59b6,stroke:#8e44ad,color:#fff
    style F fill:#f39c12,stroke:#e67e22,color:#fff
    style G fill:#f39c12,stroke:#e67e22,color:#fff
    style H fill:#f39c12,stroke:#e67e22,color:#fff
    style I fill:#f39c12,stroke:#e67e22,color:#fff
    style J fill:#f39c12,stroke:#e67e22,color:#fff
    style K fill:#f39c12,stroke:#e67e22,color:#fff
    style L fill:#e74c3c,stroke:#c0392b,color:#fff
    style M fill:#2ecc71,stroke:#27ae60,color:#fff
```

## 5. Command Usage Flowchart

```mermaid
flowchart TD
    A[City2TABULA Commands] --> B{Choose Command}

    B -->|--help| C[Show Help<br/>Display available commands<br/>and usage information]

    B -->|--create_db| D[Database Setup<br/>• Create CityDB schemas lod2, lod3<br/>• Create training and tabula schemas<br/>• Import supplementary TABULA data<br/>• Set up database functions]

    B -->|--reset_db| E[Complete Reset<br/>• Drop CityDB schemas<br/>• Drop training schemas<br/>• Recreate all schemas<br/>⚠️ Requires CityGML re-import]

    B -->|--reset_City2TABULA| F[Partial Reset<br/>• Drop only City2TABULA schema<br/>• Drop only tabula schema<br/>• Preserve CityDB data<br/>✅ Good for development]

    B -->|--extract_features| G[Feature Extraction<br/>• Query building IDs from CityDB<br/>• Create processing batches<br/>• Run parallel feature extraction<br/>• Populate training tables]

    H[External: Import CityGML<br/>Use CityDB Importer/Exporter<br/>to load building data into<br/>LOD2 and LOD3 schemas]

    D --> H
    E --> H
    H --> G
    F --> G

    G --> I[Training Data Ready<br/>ML-ready building features<br/>for TABULA classification]

    style A fill:#34495e,stroke:#2c3e50,color:#fff
    style C fill:#95a5a6,stroke:#7f8c8d,color:#fff
    style D fill:#3498db,stroke:#2980b9,color:#fff
    style E fill:#e67e22,stroke:#d35400,color:#fff
    style F fill:#f39c12,stroke:#e67e22,color:#fff
    style G fill:#27ae60,stroke:#229954,color:#fff
    style H fill:#8e44ad,stroke:#7d3c98,color:#fff
    style I fill:#2ecc71,stroke:#27ae60,color:#fff
```

## Usage Instructions

To use these Mermaid diagrams:

1. **In GitHub/GitLab**: Paste the code directly in markdown files - they'll render automatically
2. **In Documentation**: Use mermaid code blocks in your .md or .rst files
3. **Online Editor**: Use [Mermaid Live Editor](https://mermaid.live/) to preview and export
4. **VS Code**: Install the Mermaid Preview extension
5. **Export**: Generate SVG/PNG from the online editor for static documentation

## Benefits of Mermaid Diagrams

- **Version Controlled**: Diagrams as code in your repository
- **Easy Updates**: Text-based, easy to modify and maintain
- **Consistent Styling**: Automatic professional appearance
- **Collaborative**: Easy to review changes in pull requests
- **Multi-format**: Can export to SVG, PNG, PDF when needed