// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';
import type {Group} from '@mattermost/types/groups';
import type {Scheme} from '@mattermost/types/schemes';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelDetails from './channel_details';

jest.mock('utils/browser_history', () => ({
    getHistory: jest.fn(() => ({push: jest.fn(), listen: jest.fn()})),
}));

jest.mock('./channel_members', () => () => <div>{'ChannelMembers'}</div>);

jest.mock('./channel_attributes_settings', () => (props: {onChange: (values: Record<string, string>) => void}) => (
    <button onClick={() => props.onChange({field1: 'value1'})}>{'EditChannelAttribute'}</button>
));

const mockChannelLevelAccessRules = jest.fn();
jest.mock('./channel_level_access_rules', () => {
    const reactLib = require('react');
    return (props: any) => {
        mockChannelLevelAccessRules(props);
        return reactLib.createElement('div', {'data-testid': 'channel-level-access-rules'});
    };
});

describe('admin_console/team_channel_settings/channel/ChannelDetails', () => {
    const groups: Group[] = [{
        id: '123',
        name: 'name',
        display_name: 'DN',
        description: 'descript',
        source: 'A',
        remote_id: 'id',
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        has_syncables: false,
        member_count: 3,
        scheme_admin: false,
        allow_reference: false,
    }];
    const allGroups = {
        123: groups[0],
    };
    const testChannel: Channel & {team_name: string} = {
        id: '123',
        team_name: 'team',
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        team_id: 'id_123',
        type: 'O',
        display_name: 'name',
        name: 'DN',
        header: 'header',
        purpose: 'purpose',
        last_post_at: 0,
        last_root_post_at: 0,
        creator_id: 'id',
        scheme_id: 'id',
        group_constrained: false,
    };
    const team = TestHelper.getTeamMock({
        display_name: 'test',
    });
    const teamScheme: Scheme = {
        id: 'asdf',
        name: 'asdf',
        description: 'asdf',
        display_name: 'asdf',
        create_at: 0,
        update_at: 0,
        delete_at: 0,
        scope: 'team',
        default_team_admin_role: 'asdf',
        default_team_user_role: 'asdf',
        default_team_guest_role: 'asdf',
        default_channel_admin_role: 'asdf',
        default_channel_user_role: 'asdf',
        default_channel_guest_role: 'asdf',
        default_playbook_admin_role: 'asdf',
        default_playbook_member_role: 'asdf',
        default_run_member_role: 'asdf',
    };

    const actions = {
        getChannel: jest.fn().mockResolvedValue([]),
        getTeam: jest.fn().mockResolvedValue([]),
        linkGroupSyncable: jest.fn(),
        conver: jest.fn(),
        patchChannel: jest.fn(),
        setNavigationBlocked: jest.fn(),
        unlinkGroupSyncable: jest.fn(),
        getGroups: jest.fn().mockResolvedValue([]),
        membersMinusGroupMembers: jest.fn(),
        updateChannelPrivacy: jest.fn(),
        patchGroupSyncable: jest.fn(),
        getChannelModerations: jest.fn().mockResolvedValue([]),
        patchChannelModerations: jest.fn(),
        loadScheme: jest.fn(),
        addChannelMember: jest.fn(),
        removeChannelMember: jest.fn(),
        updateChannelMemberSchemeRoles: jest.fn(),
        deleteChannel: jest.fn(),
        unarchiveChannel: jest.fn(),
        getAccessControlPolicy: jest.fn().mockResolvedValue({data: null}),
        deleteAccessControlPolicy: jest.fn(),
        assignChannelToAccessControlPolicy: jest.fn(),
        unassignChannelsFromAccessControlPolicy: jest.fn(),
        searchPolicies: jest.fn(),

        // Channel-level access rules actions
        getAccessControlFields: jest.fn().mockResolvedValue({data: []}),
        getVisualAST: jest.fn().mockResolvedValue({data: {}}),
        saveChannelAccessPolicy: jest.fn().mockResolvedValue({data: {}}),
        validateChannelExpression: jest.fn().mockResolvedValue({data: {}}),
        createAccessControlSyncJob: jest.fn().mockResolvedValue({data: {}}),
        updateAccessControlPoliciesActive: jest.fn().mockResolvedValue({data: {}}),
        searchUsersForExpression: jest.fn().mockResolvedValue({data: {users: [], total: 0}}),
        getChannelMembers: jest.fn().mockResolvedValue({data: []}),
        getProfilesByIds: jest.fn().mockResolvedValue({data: []}),
        patchChannelAttributeValues: jest.fn().mockResolvedValue({data: []}),
    };

    test('should match snapshot', () => {
        const additionalProps = {
            channelPermissions: [],
            guestAccountsEnabled: true,
            channelModerationEnabled: true,
            channelGroupsEnabled: true,
            abacSupported: true,
            isDisabled: false,
        };

        let container;
        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={team}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();

        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={undefined}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();
    });
    test('should match snapshot for Professional', () => {
        const additionalProps = {
            channelPermissions: [],
            guestAccountsEnabled: true,
            channelModerationEnabled: true,
            channelGroupsEnabled: false,
            isDisabled: false,
            abacSupported: false,
        };

        let container;
        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={team}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();

        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={undefined}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();
    });

    test('should match snapshot for Enterprise', () => {
        const additionalProps = {
            channelPermissions: [],
            guestAccountsEnabled: true,
            channelModerationEnabled: true,
            channelGroupsEnabled: false,
            isDisabled: false,
            abacSupported: true,
        };

        let container;
        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={team}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();

        ({container} = renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={undefined}
                totalGroups={groups.length}
                actions={actions}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                {...additionalProps}
            />,
        ));
        expect(container).toMatchSnapshot();
    });

    test('excludes session attributes from the channel-level access rules editor', async () => {
        mockChannelLevelAccessRules.mockClear();

        const getAccessControlFields = jest.fn().mockResolvedValue({
            data: [
                {id: 'u1', name: 'department', group_id: 'cpa9q4w7m2x5c8v1b6n3k0jr5h', object_type: 'user', attrs: {managed: 'admin'}},
                {id: 's1', name: 'network_name', group_id: 'nkpkzni6yjrjt8uktpbwkagoth', object_type: 'session', target_type: 'system', attrs: {}},
            ],
        });

        const enforcedChannel = {...testChannel, policy_enforced: true};

        renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={team}
                totalGroups={groups.length}
                actions={{...actions, getAccessControlFields}}
                channel={enforcedChannel}
                channelID={enforcedChannel.id}
                allGroups={allGroups}
                channelPermissions={[]}
                guestAccountsEnabled={true}
                channelModerationEnabled={true}
                channelGroupsEnabled={false}
                abacSupported={true}
                isDisabled={false}
            />,
        );

        await waitFor(() => {
            const calls = mockChannelLevelAccessRules.mock.calls;
            const lastCall = calls[calls.length - 1][0];
            expect(lastCall.userAttributes.map((attr: {name: string}) => attr.name)).toContain('department');
        });

        const calls = mockChannelLevelAccessRules.mock.calls;
        const lastCall = calls[calls.length - 1][0];
        expect(lastCall.userAttributes.map((attr: {name: string}) => attr.name)).not.toContain('network_name');
    });

    test('stops the save at a failed attribute patch so later steps cannot replace its error', async () => {
        const patchChannelAttributeValues = jest.fn().mockResolvedValue({error: {message: 'attribute save failed'}});
        const patchChannel = jest.fn().mockResolvedValue({error: {message: 'channel patch failed'}});
        const setNavigationBlocked = jest.fn();

        renderWithContext(
            <ChannelDetails
                teamScheme={teamScheme}
                groups={groups}
                team={team}
                totalGroups={groups.length}
                actions={{...actions, patchChannelAttributeValues, patchChannel, setNavigationBlocked}}
                channel={testChannel}
                channelID={testChannel.id}
                allGroups={allGroups}
                channelPermissions={[]}
                guestAccountsEnabled={true}
                channelModerationEnabled={false}
                channelGroupsEnabled={false}
                abacSupported={false}
                channelAttributesEnabled={true}
                isDisabled={false}
            />,
        );

        await userEvent.click(screen.getByRole('button', {name: 'EditChannelAttribute'}));
        await userEvent.click(screen.getByTestId('saveSetting'));

        await waitFor(() => expect(screen.getByText('attribute save failed')).toBeInTheDocument());
        expect(patchChannelAttributeValues).toHaveBeenCalledWith(testChannel.id, [{field_id: 'field1', value: 'value1'}]);
        expect(patchChannel).not.toHaveBeenCalled();
        expect(setNavigationBlocked).toHaveBeenLastCalledWith(true);
    });
});
