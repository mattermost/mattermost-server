// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX, useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';
import {Link} from 'react-router-dom';

import type {ClientError} from '@mattermost/client';
import {ChevronLeftIcon, OpenInNewIcon, SortAscendingIcon} from '@mattermost/compass-icons/components';
import {Button} from '@mattermost/compass-ui/components/button';
import {Icon} from '@mattermost/compass-ui/components/icon';
import type {PropertyField, PropertyFieldOption} from '@mattermost/types/properties';

import {Client4} from 'mattermost-redux/client';
import {ACCESS_CONTROL_PROPERTY_GROUP, CHANNEL_OBJECT_TYPE} from 'mattermost-redux/constants/properties';
import {formatPropertyFieldLabel} from 'mattermost-redux/utils/property_utils';
import {getContrastingSimpleColor} from 'mattermost-redux/utils/theme_utils';

import {setNavigationBlocked} from 'actions/admin_actions';

import BlockableLink from 'components/admin_console/blockable_link';
import {
    CLASSIFICATIONS_MARKINGS_ADMIN_URL,
    fetchChannelClassificationField,
    fetchClassificationField,
    optionsToLevels,
    saveDeleteChannelLinkedField,
} from 'components/admin_console/classification_markings/utils';
import LoadingScreen from 'components/loading_screen';
import SaveButton from 'components/save_button';
import AdminHeader from 'components/widgets/admin_console/admin_header';
import AdminPanel from 'components/widgets/admin_console/admin_panel';
import Input from 'components/widgets/inputs/input/input';

import {useChannelResourceRemove} from './remove_channel_resource_modal';

import AppliesToCard from '../applies_to/applies_to_card';
import {buildChannelFieldPatch, buildChannelFieldPayload, parseChannelFieldConfig} from '../applies_to/channels';
import type {ChannelResourceConfig} from '../applies_to/channels';
import type {ResourceObjectType} from '../attribute_details/attribute_applies_to_constants';
import {GLOBAL_ATTRIBUTES_LIST_ROUTE} from '../constants';
import useAllowedResourceTypes from '../use_allowed_resource_types';
import {fetchLinkedFieldsForTemplate, isResourceObjectType} from '../utils';

import './classification_attribute.scss';

export const CLASSIFICATION_ATTRIBUTE_ROUTE = `${GLOBAL_ATTRIBUTES_LIST_ROUTE}/classification`;

type LoadState = 'loading' | 'ready' | 'missing' | 'failed' | 'conflict';

// The property routes answer 404 for a field that does not exist, which both of this
// page's loads treat as absent rather than broken.
function rethrowUnlessNotFound(error: unknown): undefined {
    if ((error as ClientError | undefined)?.status_code === 404) {
        return undefined;
    }
    throw error;
}

type Props = {
    disabled?: boolean;
};

/**
 * Classification's own attribute page.
 *
 * It exists because classification is the one attribute whose channel settings have
 * nowhere else to live: its fields are created by the Classification Markings page,
 * so the create-only card on the New attribute page can never reach them.
 *
 * The definition is deliberately read-only here. Levels, colors and ranks stay on
 * the Classification Markings page — one editor, not two.
 */
export default function ClassificationAttribute({disabled = false}: Props): JSX.Element {
    const dispatch = useDispatch();
    const {formatMessage} = useIntl();
    const {promptRemove} = useChannelResourceRemove();
    const allowedResourceTypes = useAllowedResourceTypes();

    const [loadState, setLoadState] = useState<LoadState>('loading');
    const [template, setTemplate] = useState<PropertyField | null>(null);

    // A same-named field classification does not own. Editing here would either
    // describe it wrongly or collide with it on save, so the page stops.
    const [conflictField, setConflictField] = useState<PropertyField | null>(null);

    // The field as the server currently holds it, so Save knows whether to create,
    // patch or delete rather than inferring it from the form.
    const [channelField, setChannelField] = useState<PropertyField | null>(null);

    // null means classification does not apply to channels.
    const [channelResource, setChannelResource] = useState<ChannelResourceConfig | null>(null);

    // The template's other linked fields. Classification Markings applies the
    // template to Users under a different name (`clearance`), and Attribute
    // Management lists no row for a linked field, so this page is the only
    // place that name can be found.
    const [otherLinkedResources, setOtherLinkedResources] = useState<Array<{type: ResourceObjectType; name: string}>>([]);

    const [saving, setSaving] = useState(false);
    const [saveFailed, setSaveFailed] = useState(false);
    const [serverErrorMessage, setServerErrorMessage] = useState<string | null>(null);

    // Gates Save, so an idle click cannot rewrite a field with what it already holds.
    const [dirty, setDirty] = useState(false);

    const isMountedRef = useRef(true);
    useEffect(() => () => {
        isMountedRef.current = false;
    }, []);

    useEffect(() => {
        (async () => {
            try {
                // 404 is how the property routes say "no such field", which is an
                // ordinary state for both of these: classification may not be set up,
                // and it may be set up without applying to channels. Only a real
                // failure gets the error state.
                const templateLookup = (await fetchClassificationField().catch(rethrowUnlessNotFound)) ?? {};
                if (!isMountedRef.current) {
                    return;
                }
                if (templateLookup.conflict) {
                    setConflictField(templateLookup.conflict);
                    setLoadState('conflict');
                    return;
                }

                const templateField = templateLookup.field;
                if (!templateField) {
                    setLoadState('missing');
                    return;
                }

                const channelLookup = (await fetchChannelClassificationField(templateField.id).catch(rethrowUnlessNotFound)) ?? {};
                if (!isMountedRef.current) {
                    return;
                }
                if (channelLookup.conflict) {
                    setConflictField(channelLookup.conflict);
                    setLoadState('conflict');
                    return;
                }

                const existingChannelField = channelLookup.field;
                setTemplate(templateField);
                setChannelField(existingChannelField ?? null);
                setChannelResource(existingChannelField ? parseChannelFieldConfig(existingChannelField) : null);
                setLoadState('ready');
            } catch (error) {
                // Logged as well as shown: the canned copy cannot say which call failed.
                console.error('ClassificationAttribute-load: ', error); // eslint-disable-line no-console
                if (isMountedRef.current) {
                    setServerErrorMessage((error as ClientError | undefined)?.message ?? null);
                    setLoadState('failed');
                }
            }
        })();
    }, []);

    // Kept out of the load above, whose catch replaces the whole page with an
    // error screen. These rows only report which other resources the template
    // reaches, so a scope that will not list costs the rows, not the page.
    useEffect(() => {
        if (!template) {
            return undefined;
        }

        let cancelled = false;

        (async () => {
            try {
                const linkedFields = (await fetchLinkedFieldsForTemplate(template.id, allowedResourceTypes).catch(rethrowUnlessNotFound)) ?? [];
                if (cancelled) {
                    return;
                }
                const otherResources: Array<{type: ResourceObjectType; name: string}> = [];
                for (const linked of linkedFields) {
                    if (linked.object_type !== CHANNEL_OBJECT_TYPE && isResourceObjectType(linked.object_type)) {
                        otherResources.push({type: linked.object_type, name: linked.name});
                    }
                }
                setOtherLinkedResources(otherResources);
            } catch (error) {
                console.error('ClassificationAttribute-load-linked-resources: ', error); // eslint-disable-line no-console
            }
        })();

        return () => {
            cancelled = true;
        };
    }, [allowedResourceTypes, template]);

    const levels = useMemo(() => {
        const options = (template?.attrs?.options ?? []) as PropertyFieldOption[];
        return optionsToLevels(options);
    }, [template]);

    const markDirty = useCallback(() => {
        setDirty(true);
        dispatch(setNavigationBlocked(true));
        setSaveFailed(false);
        setServerErrorMessage(null);
    }, [dispatch]);

    const markClean = useCallback(() => {
        setDirty(false);
        dispatch(setNavigationBlocked(false));
    }, [dispatch]);

    const handleChannelResourceChange = useCallback((next: ChannelResourceConfig | null) => {
        setChannelResource(next);
        markDirty();
    }, [markDirty]);

    // Removal is destructive on this page in a way it is not on the New attribute
    // page: there the resource has not been saved yet, here the field exists and its
    // values go with it. So it asks, and only a saved field needs asking about.
    const handleChannelResourceRemove = useCallback(async () => {
        if (!channelField) {
            handleChannelResourceChange(null);
            return;
        }
        if (!await promptRemove()) {
            return;
        }
        setSaving(true);
        setSaveFailed(false);
        try {
            await saveDeleteChannelLinkedField(channelField.id);
            if (!isMountedRef.current) {
                return;
            }
            setChannelField(null);
            setChannelResource(null);
            markClean();
        } catch (error) {
            if (isMountedRef.current) {
                setSaveFailed(true);
                setServerErrorMessage((error as ClientError | undefined)?.message ?? null);
            }
        } finally {
            if (isMountedRef.current) {
                setSaving(false);
            }
        }
    }, [channelField, handleChannelResourceChange, markClean, promptRemove]);

    const canSave = dirty && !saving && !disabled;

    const handleSave = useCallback(async () => {
        if (!template || !canSave) {
            return;
        }
        setSaving(true);
        setSaveFailed(false);
        try {
            if (channelResource && channelField) {
                const patched = await Client4.patchPropertyField(
                    ACCESS_CONTROL_PROPERTY_GROUP,
                    CHANNEL_OBJECT_TYPE,
                    channelField.id,
                    buildChannelFieldPatch(channelResource),
                );
                if (!isMountedRef.current) {
                    return;
                }
                setChannelField(patched);
                setChannelResource(parseChannelFieldConfig(patched));
            } else if (channelResource) {
                const created = await Client4.createPropertyField(
                    ACCESS_CONTROL_PROPERTY_GROUP,
                    CHANNEL_OBJECT_TYPE,
                    buildChannelFieldPayload(template, channelResource),
                );
                if (!isMountedRef.current) {
                    return;
                }
                setChannelField(created);
                setChannelResource(parseChannelFieldConfig(created));
            }

            if (!isMountedRef.current) {
                return;
            }
            markClean();
        } catch (error) {
            if (isMountedRef.current) {
                setSaveFailed(true);
                setServerErrorMessage((error as ClientError | undefined)?.message ?? null);
            }
        } finally {
            if (isMountedRef.current) {
                setSaving(false);
            }
        }
    }, [canSave, channelField, channelResource, markClean, template]);

    const displayName = (template?.attrs?.display_name as string | undefined)?.trim() ||
        formatPropertyFieldLabel(template?.name ?? formatMessage(messages.nameFallback));

    return (
        <div
            className='wrapper--fixed ClassificationAttribute'
            data-testid='classificationAttribute'
        >
            <AdminHeader withBackButton={true}>
                <div>
                    <div className='ClassificationAttribute__back'>
                        <BlockableLink
                            to={GLOBAL_ATTRIBUTES_LIST_ROUTE}
                            className='ClassificationAttribute__backButton'
                            aria-label={formatMessage(messages.backLink)}
                            data-testid='classificationAttributeBackLink'
                        >
                            <ChevronLeftIcon
                                size={28}
                                aria-hidden={true}
                            />
                        </BlockableLink>
                    </div>
                    <hgroup className='ClassificationAttribute__headerGroup'>
                        <FormattedMessage
                            tagName='h1'
                            {...messages.title}
                            values={{name: displayName}}
                        />
                        <FormattedMessage
                            tagName='p'
                            {...messages.subtitle}
                        />
                    </hgroup>
                </div>
            </AdminHeader>
            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    {loadState === 'loading' && <LoadingScreen/>}
                    {loadState === 'missing' && (
                        <p
                            className='ClassificationAttribute__notice'
                            data-testid='classificationAttributeMissing'
                        >
                            <FormattedMessage
                                {...messages.notConfigured}
                                values={{link: (
                                    <Link to={CLASSIFICATIONS_MARKINGS_ADMIN_URL}>
                                        <FormattedMessage {...messages.markingsPageName}/>
                                    </Link>
                                )}}
                            />
                        </p>
                    )}
                    {/* The name is not reserved, so Attribute Management can create an
                        attribute called `classification` that this page does not own.
                        Two ways in: a template of any type other than rank (the listing
                        no longer links here for one, but the URL still resolves), or a
                        channel attribute of that name linked to a different template,
                        whose options the Applies-to card below would otherwise edit.
                        Both are cleared by renaming or deleting the attribute. */}
                    {loadState === 'conflict' && conflictField && (
                        <p
                            className='ClassificationAttribute__notice'
                            role='alert'
                            data-testid='classificationAttributeConflict'
                        >
                            <FormattedMessage
                                {...messages.nameConflict}
                                values={{name: conflictField.name}}
                            />
                        </p>
                    )}
                    {loadState === 'failed' && (
                        <p
                            className='ClassificationAttribute__notice'
                            role='alert'
                            data-testid='classificationAttributeLoadError'
                        >
                            {serverErrorMessage ?? formatMessage(messages.loadFailed)}
                        </p>
                    )}
                    {loadState === 'ready' && template && (
                        <>
                            <AdminPanel
                                id='classification_attribute_definition'
                                title={messages.definitionTitle}
                                subtitle={messages.definitionSubtitle}
                            >
                                <div className='ClassificationAttribute__row'>
                                    <label
                                        className='ClassificationAttribute__label'
                                        htmlFor='input_display_name'
                                    >
                                        <FormattedMessage {...messages.displayNameLabel}/>
                                    </label>
                                    <div className='ClassificationAttribute__fieldControl'>
                                        <Input
                                            name='display_name'
                                            type='text'
                                            useLegend={false}
                                            aria-label={formatMessage(messages.displayNameLabel)}
                                            value={displayName}
                                            disabled={true}
                                            data-testid='classificationAttributeName'
                                        />
                                        <div className='ClassificationAttribute__uniqueName'>
                                            <span className='ClassificationAttribute__uniqueNameCaption'>
                                                <span className='ClassificationAttribute__uniqueNamePrefix'>
                                                    <FormattedMessage {...messages.uniqueNamePrefix}/>
                                                </span>
                                                <span data-testid='classificationAttributeUniqueName'>
                                                    {template.name}
                                                </span>
                                            </span>
                                            <p className='ClassificationAttribute__helperText'>
                                                <FormattedMessage {...messages.helperText}/>
                                            </p>
                                        </div>
                                    </div>
                                </div>
                                <div className='ClassificationAttribute__row'>
                                    <span className='ClassificationAttribute__label'>
                                        <FormattedMessage {...messages.typeLabel}/>
                                    </span>
                                    <div className='ClassificationAttribute__fieldControl'>
                                        <button
                                            type='button'
                                            className='ClassificationAttribute__typeButton'
                                            disabled={true}
                                            aria-label={formatMessage(messages.typeFieldAriaLabel, {value: formatMessage(messages.typeRanked)})}
                                            data-testid='classificationAttributeType'
                                        >
                                            <span className='ClassificationAttribute__typeButtonInner'>
                                                <SortAscendingIcon size={18}/>
                                                <FormattedMessage {...messages.typeRanked}/>
                                            </span>
                                            <i className='icon icon-chevron-down'/>
                                        </button>
                                    </div>
                                </div>
                                <div className='ClassificationAttribute__row'>
                                    <span className='ClassificationAttribute__label'>
                                        <FormattedMessage {...messages.optionsLabel}/>
                                    </span>
                                    <div className='ClassificationAttribute__fieldControl'>
                                        <ul
                                            className='ClassificationAttribute__options'
                                            data-testid='classificationAttributeLevels'
                                        >
                                            {levels.map((level) => (
                                                <li key={level.id}>
                                                    <span
                                                        className='ClassificationAttribute__optionChip'
                                                        style={{
                                                            backgroundColor: level.color,
                                                            color: getContrastingSimpleColor(level.color) || '#FFFFFF',
                                                        }}
                                                    >
                                                        <span
                                                            className='ClassificationAttribute__optionRankShade'
                                                            aria-hidden={true}
                                                        />
                                                        <span className='ClassificationAttribute__optionRank'>
                                                            {level.rank}
                                                        </span>
                                                        <span className='ClassificationAttribute__optionLabel'>
                                                            {level.name}
                                                        </span>
                                                    </span>
                                                </li>
                                            ))}
                                        </ul>
                                        <div className='ClassificationAttribute__markingsFooter'>
                                            <div className='ClassificationAttribute__markingsCopy'>
                                                <FormattedMessage {...messages.markingsFooter}/>
                                                <Button
                                                    asChild={true}
                                                    emphasis='tertiary'
                                                    size='small'
                                                    className='ClassificationAttribute__markingsOpen'
                                                    data-testid='classificationAttributeMarkingsLink'
                                                    trailingIcon={<Icon glyph={<OpenInNewIcon/>}/>}
                                                >
                                                    <Link to={CLASSIFICATIONS_MARKINGS_ADMIN_URL}>
                                                        <FormattedMessage {...messages.openMarkings}/>
                                                    </Link>
                                                </Button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </AdminPanel>
                            <AppliesToCard
                                ordered={true}
                                readOnlyResources={otherLinkedResources}
                                channelResource={channelResource}
                                onChannelResourceChange={handleChannelResourceChange}
                                onChannelResourceRemove={handleChannelResourceRemove}
                                disabled={saving || disabled}
                                channelFieldId={channelField?.id}
                                attributeDisplayName={displayName}
                                channelFieldUpdateAt={channelField?.update_at}
                            />
                        </>
                    )}
                </div>
            </div>
            {loadState === 'ready' && (
                <div className='admin-console-save'>
                    <SaveButton
                        disabled={!canSave}
                        saving={saving}
                        onClick={handleSave}
                        defaultMessage={<FormattedMessage {...messages.save}/>}
                    />
                    <Button
                        asChild={true}
                        emphasis='tertiary'
                        data-testid='classificationAttributeCancelLink'
                    >
                        <BlockableLink to={GLOBAL_ATTRIBUTES_LIST_ROUTE}>
                            <FormattedMessage {...messages.cancel}/>
                        </BlockableLink>
                    </Button>
                    {saveFailed && (
                        <span
                            className='ClassificationAttribute__error'
                            role='alert'
                            data-testid='classificationAttributeSaveError'
                        >
                            <i className='icon icon-alert-outline'/>
                            {serverErrorMessage ?? formatMessage(messages.saveFailed)}
                        </span>
                    )}
                </div>
            )}
        </div>
    );
}

const messages = defineMessages({
    backLink: {id: 'admin.global_attributes.classification.back_link', defaultMessage: 'Back to Attribute Management'},
    title: {id: 'admin.global_attributes.attribute_details.edit_title', defaultMessage: 'Edit {name} Attribute'},
    nameFallback: {id: 'admin.global_attributes.classification.name_fallback', defaultMessage: 'Classification'},
    subtitle: {
        id: 'admin.global_attributes.classification.subtitle',
        defaultMessage: 'Choose the resources this attribute applies to, and how it behaves on each.',
    },
    definitionTitle: {id: 'admin.global_attributes.attribute_details.definition.title', defaultMessage: 'Definition'},
    definitionSubtitle: {
        id: 'admin.global_attributes.attribute_details.definition.subtitle',
        defaultMessage: 'Display name, type, and options.',
    },
    displayNameLabel: {id: 'admin.global_attributes.attribute_details.display_name.label', defaultMessage: 'Display name'},
    uniqueNamePrefix: {id: 'admin.global_attributes.attribute_details.unique_name.prefix', defaultMessage: 'Unique name:'},
    helperText: {
        id: 'admin.global_attributes.attribute_details.unique_name.helper_text',
        defaultMessage: 'Name is the internal identifier for policies and integrations. Display name is what admins and users see.',
    },
    typeLabel: {id: 'admin.global_attributes.attribute_details.type.label', defaultMessage: 'Type'},
    typeRanked: {id: 'admin.global_attributes.table.type.rank', defaultMessage: 'Ranked'},
    typeFieldAriaLabel: {id: 'admin.global_attributes.attribute_details.type.field_aria_label', defaultMessage: 'Type: {value}'},
    optionsLabel: {id: 'admin.global_attributes.attribute_details.options.label', defaultMessage: 'Options'},
    markingsFooter: {
        id: 'admin.global_attributes.classification.markings_footer',
        defaultMessage: 'Presets and marking colors are configured on Classification Markings.',
    },
    openMarkings: {id: 'admin.global_attributes.classification.open_markings', defaultMessage: 'Open'},
    markingsPageName: {
        id: 'admin.global_attributes.classification.markings_page_name',
        defaultMessage: 'Classification Markings',
    },
    notConfigured: {
        id: 'admin.global_attributes.classification.not_configured',
        defaultMessage: 'Classification is not set up yet. Enable it on the {link} page first.',
    },
    nameConflict: {
        id: 'admin.global_attributes.classification.name_conflict',
        defaultMessage: 'An attribute named "{name}" already exists but is not part of classification. Rename or remove it in Attribute Management, then reload this page.',
    },
    loadFailed: {
        id: 'admin.global_attributes.classification.load_failed',
        defaultMessage: 'Something went wrong while loading classification. Please try again.',
    },
    saveFailed: {
        id: 'admin.global_attributes.classification.save_failed',
        defaultMessage: 'Something went wrong while saving. Please try again.',
    },
    save: {id: 'admin.global_attributes.classification.save', defaultMessage: 'Save'},
    cancel: {id: 'admin.global_attributes.classification.cancel', defaultMessage: 'Cancel'},
});
