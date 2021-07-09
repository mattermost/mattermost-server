#!/bin/bash

export COMPOSE_PROJECT_NAME=localdev
local_cmdname=${0##*/}

usage()
{
    cat << USAGE >&2
Usage:
    $local_cmdname up/down
USAGE
    exit 1
}

up()
{
    docker-compose run --rm start_dependencies

    docker-compose exec openldap bash -c 'echo -e "dn: ou=testusers,dc=mm,dc=test,dc=com\nobjectclass: organizationalunit" | ldapadd -x -D "cn=admin,dc=mm,dc=test,dc=com" -w mostest'
    docker-compose exec openldap bash -c 'echo -e "dn: uid=test.one,ou=testusers,dc=mm,dc=test,dc=com\nobjectclass: iNetOrgPerson\nsn: User\ncn: Test1\nmail: success+testone@simulator.amazonses.com" | ldapadd -x -D "cn=admin,dc=mm,dc=test,dc=com" -w mostest'
    docker-compose exec openldap bash -c 'ldappasswd -s Password1 -D "cn=admin,dc=mm,dc=test,dc=com" -x "uid=test.one,ou=testusers,dc=mm,dc=test,dc=com" -w mostest'
    docker-compose exec openldap bash -c 'echo -e "dn: uid=test.two,ou=testusers,dc=mm,dc=test,dc=com\nobjectclass: iNetOrgPerson\nsn: User\ncn: Test2\nmail: success+testtwo@simulator.amazonses.com" | ldapadd -x -D "cn=admin,dc=mm,dc=test,dc=com" -w mostest'
    docker-compose exec openldap bash -c 'ldappasswd -s Password1 -D "cn=admin,dc=mm,dc=test,dc=com" -x "uid=test.two,ou=testusers,dc=mm,dc=test,dc=com" -w mostest'
    docker-compose exec openldap bash -c 'echo -e "dn: cn=tgroup,ou=testusers,dc=mm,dc=test,dc=com\nobjectclass: groupOfUniqueNames\nuniqueMember: uid=test.one,ou=testusers,dc=mm,dc=test,dc=com" | ldapadd -x -D "cn=admin,dc=mm,dc=test,dc=com" -w mostest'

    docker run -it -u root \
        --privileged \
        -v $GOPATH:/go \
        -w /go/src/github.com/mattermost/mattermost-server/ \
        --net ${COMPOSE_PROJECT_NAME}_mm-test \
        --env-file=dotenv/test.env
        -e GOPATH="/go" \
        -e MM_SQLSETTINGS_DATASOURCE="postgres://mmuser:mostest@postgres:5432/mattermost_test?sslmode=disable&connect_timeout=10" \
        -e MM_SQLSETTINGS_DRIVERNAME=postgres
        mattermost/mattermost-build-server:20210709_golang-1.16.5 bash
}

down()
{
    docker-compose down
}

# process arguments
while [[ $# -gt 0 ]]
do
    case "$1" in
        up)
        echo "Starting Containers"
        up
        break
        ;;

        down)
        echo "Stopping Containers"
        down
        break
        ;;

        --help)
        usage
        ;;

        *)
        echo "Unknown argument: $1" >&2
        usage
        ;;
    esac
done

if [[ "$1" == "" ]]; then
    usage
fi
