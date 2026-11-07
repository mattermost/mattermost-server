// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import moment from 'moment';
import React from 'react';
import {Modal} from 'react-bootstrap';
import {FormattedMessage} from 'react-intl';
import {Link} from 'react-router-dom';

import {extractFilenameFromContentDisposition} from '@mattermost/client';
import {DownloadOutlineIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {Checkbox} from '@mattermost/compass-ui/components/checkbox';
import {Icon} from '@mattermost/compass-ui/components/icon';
import type {SupportPacketContent} from '@mattermost/types/admin';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';

import AlertBanner from 'components/alert_banner';
import ExternalLink from 'components/external_link';

import './commercial_support_modal.scss';

type Props = {

    /**
     * Function called after the modal has been hidden
     */
    onExited: () => void;

    showBannerWarning: boolean;

    isCloud: boolean;

    currentUser: UserProfile;

    packetContents: SupportPacketContent[];
};

type State = {
    show: boolean;
    showBannerWarning: boolean;
    packetContents: SupportPacketContent[];
    loading: boolean;
    error?: string;
};

export default class CommercialSupportModal extends React.PureComponent<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = {
            show: true,
            showBannerWarning: props.showBannerWarning,
            packetContents: props.packetContents,
            loading: false,
        };
    }

    componentDidUpdate = (prevProps: Props) => {
        if (this.props.showBannerWarning !== prevProps.showBannerWarning) {
            this.updateBannerWarning(this.props.showBannerWarning);
        }
    };

    doHide = () => {
        this.setState({show: false});
    };

    updateBannerWarning = (showBannerWarning: boolean) => {
        this.setState({showBannerWarning});
    };

    hideBannerWarning = () => {
        this.updateBannerWarning(false);
    };

    updateCheckStatus = (index: number) => {
        this.setState({
            packetContents: this.state.packetContents.map((content, currentIndex) => (
                (currentIndex === index && !content.mandatory) ? {...content, selected: !content.selected} : content
            )),
        });
    };

    genereateDownloadURLWithParams = (): string => {
        const url = new URL(Client4.getSystemRoute() + '/support_packet');
        this.state.packetContents.forEach((content) => {
            if (content.id === 'basic.server.logs') {
                url.searchParams.set('basic_server_logs', String(content.selected));
            } else if (!content.mandatory && content.selected) {
                url.searchParams.append('plugin_packets', content.id);
            }
        });
        return url.toString();
    };

    extractFilename = (input: string | null): string => {
        // construct the expected filename in case of an error in the header
        const formattedDate = (moment(new Date())).format('YYYY-MM-DDTHH-mm');
        const presumedFileName = `mm_support_packet_${formattedDate}.zip`;

        return extractFilenameFromContentDisposition(input, presumedFileName);
    };

    downloadSupportPacket = async () => {
        this.setState({loading: true, error: undefined});
        const res = await fetch(this.genereateDownloadURLWithParams(), {
            method: 'GET',
            headers: {'Content-Type': 'application/zip'},
        });
        if (!res.ok) {
            const data = await res.json();
            const error = data.message + ': ' + data.detailed_error;
            this.setState({loading: false, error});
            return;
        }

        const blob = await res.blob();
        this.setState({loading: false});

        const href = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = href;
        link.setAttribute('download', this.extractFilename(res.headers.get('content-disposition')));
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    render() {
        const {showBannerWarning} = this.state;
        const {isCloud, currentUser} = this.props;

        const supportLink = isCloud ? `https://customers.mattermost.com/cloud/contact-us?name=${encodeURIComponent(currentUser.first_name + ' ' + currentUser.last_name)}&email=${encodeURIComponent(currentUser.email)}&inquiry=technical` : 'https://support.mattermost.com/hc/en-us/requests/new';
        return (
            <Modal
                id='commercialSupportModal'
                dialogClassName='a11y__modal more-modal more-direct-channels'
                show={this.state.show}
                onHide={this.doHide}
                onExited={this.props.onExited}
            >
                <Modal.Header closeButton={true}>
                    <Modal.Title>
                        <FormattedMessage
                            id='commercial_support.title'
                            defaultMessage='Commercial Support'
                        />
                    </Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <div className='CommercialSupportModal'>
                        <FormattedMessage
                            id='commercial_support_modal.description'
                            defaultMessage={'If you\'re experiencing issues, <supportLink>submit a support ticket</supportLink>. To help with troubleshooting, it\'s recommended to download the Support Packet below that includes more details about your Mattermost environment.'}
                            values={{
                                supportLink: (chunks) => (
                                    <ExternalLink
                                        href={supportLink}
                                        location='commercialSupportModal'
                                    >
                                        {chunks}
                                    </ExternalLink>
                                ),
                            }}
                        />
                        {showBannerWarning &&
                            <AlertBanner
                                mode='info'
                                message={
                                    <FormattedMessage
                                        id='commercial_support_modal.warning.banner'
                                        defaultMessage='Before downloading the Support Packet, set <strong>Output Logs to File</strong> to <strong>true</strong> and set <strong>File Log Level</strong> to <strong>DEBUG</strong> <debugLink>here</debugLink>.'
                                        values={{
                                            strong: (chunks) => <strong>{chunks}</strong>,
                                            debugLink: (chunks) => <Link to='/admin_console/environment/logging'>{chunks}</Link>,
                                        }}
                                    />
                                }
                                onDismiss={this.hideBannerWarning}
                            />
                        }
                        <div className='CommercialSupportModal__packet_contents_download'>
                            <strong>
                                <FormattedMessage
                                    id='commercial_support_modal.download_contents'
                                    defaultMessage={'Select your Support Packet contents to download'}
                                />
                            </strong>
                        </div>
                        <div className='CommercialSupportModal__options'>
                            {this.state.packetContents.map((item, index) => (
                                <div
                                    className='CommercialSupportModal__option'
                                    key={item.id}
                                >
                                    <Checkbox
                                        id={item.id}
                                        name={item.id}
                                        checked={item.selected}
                                        disabled={item.mandatory}
                                        onChange={() => this.updateCheckStatus(index)}
                                    >
                                        {item.label}
                                    </Checkbox>
                                </div>
                            ))}
                        </div>
                        <div className='CommercialSupportModal__download'>
                            {this.state.error && (
                                <div className='CommercialSupportModal__error'>
                                    <span className='error-text'>{this.state.error}</span>
                                </div>
                            )}
                            <Button
                                emphasis='primary'
                                className='DownloadSupportPacket'
                                onClick={this.downloadSupportPacket}
                                loading={this.state.loading}
                                leadingIcon={<Icon glyph={<DownloadOutlineIcon/>}/>}
                            >
                                <FormattedMessage
                                    id='commercial_support.download_support_packet'
                                    defaultMessage='Download Support Packet'
                                />
                            </Button>
                        </div>
                    </div>
                </Modal.Body>
            </Modal>
        );
    }
}
