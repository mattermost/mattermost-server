// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

import ProfilePicture from 'components/profile_picture.jsx';

import TeamStore from 'stores/team_store.jsx';
import UserStore from 'stores/user_store.jsx';
import ChannelStore from 'stores/channel_store.jsx';

import TeamMembersModal from './team_members_modal.jsx';
import ChannelMembersModal from './channel_members_modal.jsx';
import ChannelInviteModal from './channel_invite_modal.jsx';

import {openDirectChannelToUser} from 'actions/channel_actions.jsx';

import * as AsyncClient from 'utils/async_client.jsx';
import Client from 'client/web_client.jsx';
import * as Utils from 'utils/utils.jsx';
import Constants from 'utils/constants.jsx';

import $ from 'jquery';
import React from 'react';
import {Popover, Overlay} from 'react-bootstrap';
import {FormattedMessage} from 'react-intl';
import {browserHistory} from 'react-router/es6';

export default class PopoverListMembers extends React.Component {
    constructor(props) {
        super(props);

        this.showMembersModal = this.showMembersModal.bind(this);

        this.handleShowDirectChannel = this.handleShowDirectChannel.bind(this);
        this.closePopover = this.closePopover.bind(this);

        this.state = {
            showPopover: false,
            showTeamMembersModal: false,
            showChannelMembersModal: false,
            showChannelInviteModal: false
        };
    }

    componentDidUpdate() {
        $('.member-list__popover .popover-content').perfectScrollbar();
    }

    handleShowDirectChannel(teammate, e) {
        e.preventDefault();

        openDirectChannelToUser(
            teammate.id,
            (channel, channelAlreadyExisted) => {
                browserHistory.push(TeamStore.getCurrentTeamRelativeUrl() + '/channels/' + channel.name);
                if (channelAlreadyExisted) {
                    this.closePopover();
                }
            },
            () => {
                this.closePopover();
            }
        );
    }

    closePopover() {
        this.setState({showPopover: false});
    }

    showMembersModal(e) {
        e.preventDefault();

        if (ChannelStore.isDefault(this.props.channel)) {
            this.setState({
                showPopover: false,
                showTeamMembersModal: true
            });
        } else {
            this.setState({
                showPopover: false,
                showChannelMembersModal: true
            });
        }
    }

    render() {
        const popoverHtml = [];
        const members = this.props.members;
        const teamMembers = UserStore.getProfilesUsernameMap();
        let isAdmin = false;
        const currentUserId = UserStore.getCurrentId();

        isAdmin = TeamStore.isTeamAdminForCurrentTeam() || UserStore.isSystemAdminForCurrentUser();

        if (members && teamMembers) {
            members.sort((a, b) => {
                const aName = Utils.displayUsername(a.id);
                const bName = Utils.displayUsername(b.id);

                return aName.localeCompare(bName);
            });

            members.forEach((m, i) => {
                let button = '';
                if (currentUserId !== m.id && this.props.channel.type !== Constants.DM_CHANNEl) {
                    button = (
                        <a
                            href='#'
                            className='btn-message'
                            onClick={(e) => this.handleShowDirectChannel(m, e)}
                        >
                            <FormattedMessage
                                id='members_popover.msg'
                                defaultMessage='Message'
                            />
                        </a>
                    );
                }

                let name = '';
                if (teamMembers[m.username]) {
                    name = Utils.displayUsername(teamMembers[m.username].id);
                }

                if (name) {
                    popoverHtml.push(
                        <div
                            className='more-modal__row'
                            key={'popover-member-' + i}
                        >
                            <ProfilePicture
                                src={`${Client.getUsersRoute()}/${m.id}/image?time=${m.last_picture_update}`}
                                width='26'
                                height='26'
                            />
                            <div className='more-modal__details'>
                                <div
                                    className='more-modal__name'
                                >
                                    {name}
                                </div>
                            </div>
                            <div
                                className='more-modal__actions'
                            >
                                {button}
                            </div>
                        </div>
                    );
                }
            });

            if (this.props.channel.type !== Constants.GM_CHANNEL) {
                let membersName = (
                    <FormattedMessage
                        id='members_popover.manageMembers'
                        defaultMessage='Manage Members'
                    />
                );
                if (!isAdmin && ChannelStore.isDefault(this.props.channel)) {
                    membersName = (
                        <FormattedMessage
                            id='members_popover.viewMembers'
                            defaultMessage='View Members'
                        />
                    );
                }

                popoverHtml.push(
                    <div
                        className='more-modal__row'
                        key={'popover-member-more'}
                    >
                        <div className='col-sm-3'/>
                        <div className='more-modal__details'>
                            <div
                                className='more-modal__name'
                            >
                                <a
                                    href='#'
                                    onClick={this.showMembersModal}
                                >
                                    {membersName}
                                </a>
                            </div>
                        </div>
                    </div>
                );
            }
        }

        const count = this.props.memberCount;
        let countText = '-';
        if (count > 0) {
            countText = count.toString();
        }

        const title = (
            <FormattedMessage
                id='members_popover.title'
                defaultMessage='Members'
            />
        );

        let channelMembersModal;
        if (this.state.showChannelMembersModal) {
            channelMembersModal = (
                <ChannelMembersModal
                    onModalDismissed={() => this.setState({showChannelMembersModal: false})}
                    showInviteModal={() => this.setState({showChannelInviteModal: true})}
                    channel={this.props.channel}
                />
            );
        }

        let teamMembersModal;
        if (this.state.showTeamMembersModal) {
            teamMembersModal = (
                <TeamMembersModal
                    onHide={() => this.setState({showTeamMembersModal: false})}
                    isAdmin={isAdmin}
                />
            );
        }

        let channelInviteModal;
        if (this.state.showChannelInviteModal) {
            channelInviteModal = (
                <ChannelInviteModal
                    onHide={() => this.setState({showChannelInviteModal: false})}
                    channel={this.props.channel}
                />
            );
        }

        return (
            <div>
                <div
                    id='member_popover'
                    className='member-popover__trigger'
                    ref='member_popover_target'
                    onClick={(e) => {
                        this.setState({popoverTarget: e.target, showPopover: !this.state.showPopover});
                        AsyncClient.getProfilesInChannel(this.props.channel.id, 0);
                    }}
                >
                    <div>
                        {countText}
                        <span
                            className='fa fa-user'
                            aria-hidden='true'
                        />
                    </div>
                </div>
                <Overlay
                    rootClose={true}
                    onHide={this.closePopover}
                    show={this.state.showPopover}
                    target={() => this.state.popoverTarget}
                    placement='bottom'
                >
                    <Popover
                        ref='memebersPopover'
                        title={title}
                        className='member-list__popover'
                        id='member-list-popover'
                    >
                        <div className='more-modal__list'>{popoverHtml}</div>
                    </Popover>
                </Overlay>
                {channelMembersModal}
                {teamMembersModal}
                {channelInviteModal}
            </div>
        );
    }
}

PopoverListMembers.propTypes = {
    channel: React.PropTypes.object.isRequired,
    members: React.PropTypes.array.isRequired,
    memberCount: React.PropTypes.number
};
