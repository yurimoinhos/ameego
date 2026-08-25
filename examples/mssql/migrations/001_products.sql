-- MSSQL / T-SQL example with VECTOR embeddings
CREATE TABLE dbo.products (
    id          UNIQUEIDENTIFIER NOT NULL PRIMARY KEY DEFAULT NEWID(),
    sku         NVARCHAR(64) NOT NULL,
    price       DECIMAL(18, 2) NOT NULL,
    stock       INT NOT NULL DEFAULT 0,
    is_active   BIT NOT NULL DEFAULT 1,
    embedding   VECTOR(8) NULL,
    created_at  DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
