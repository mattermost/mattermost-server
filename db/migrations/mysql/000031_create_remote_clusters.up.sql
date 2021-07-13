CREATE TABLE IF NOT EXISTS RemoteClusters (
    RemoteId VARCHAR(26) NOT NULL,
    RemoteTeamId VARCHAR(26),
    Name VARCHAR(64) NOT NULL,
    DisplayName VARCHAR(64),
    SiteURL VARCHAR(168),
    CreateAt bigint,
    LastPingAt bigint,
    Token VARCHAR(26) NOT NULL,
    RemoteToken VARCHAR(26),
    Topics VARCHAR(512),
    CreatorId VARCHAR(26) NOT NULL,
    PRIMARY KEY (RemoteId, Name)
);

SET @preparedStatement = (SELECT IF(
    (
        SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
        WHERE table_name = 'RemoteClusters'
        AND table_schema = DATABASE()
        AND index_name = 'remote_clusters_site_url_unique'
    ) > 0,
    'SELECT 1',
    'CREATE UNIQUE INDEX remote_clusters_site_url_unique ON RemoteClusters (RemoteTeamId, SiteURL(168));'
));

PREPARE createIndexIfNotExists FROM @preparedStatement;
EXECUTE createIndexIfNotExists;
DEALLOCATE PREPARE createIndexIfNotExists;
