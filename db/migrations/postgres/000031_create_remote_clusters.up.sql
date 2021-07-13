CREATE TABLE IF NOT EXISTS remoteclusters (
    remoteid VARCHAR(26) NOT NULL,
    remoteteamid VARCHAR(26),
    name VARCHAR(64) NOT NULL,
    displayname VARCHAR(64),
    siteurl VARCHAR(512),
    createat bigint,
    lastpingat bigint,
    token VARCHAR(26) NOT NULL,
    remotetoken VARCHAR(26),
    topics VARCHAR(512),
    creatorid VARCHAR(26) NOT NULL,
    PRIMARY KEY (remoteid, name)
);

CREATE UNIQUE INDEX remote_clusters_site_url_unique ON remoteclusters (remoteteamid, siteurl);
