// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getRandomId} from '../../utils';
import * as TIMEOUTS from '../../fixtures/timeouts';

Cypress.Commands.add('uiCreateChannel', ({
    prefix = 'channel-',
    isPrivate = false,
    purpose = '',
    name = '',
    createBoard = '',
}) => {
    cy.uiBrowseOrCreateChannel('Create new channel');

    cy.get('#new-channel-modal').should('be.visible');
    if (isPrivate) {
        cy.get('#public-private-selector-button-P').click().wait(TIMEOUTS.HALF_SEC);
    } else {
        cy.get('#public-private-selector-button-O').click().wait(TIMEOUTS.HALF_SEC);
    }
    const channelName = name || `${prefix}${getRandomId()}`;
    cy.get('#input_new-channel-modal-name').should('be.visible').clear().type(channelName);
    if (purpose) {
        cy.get('#new-channel-modal-purpose').clear().type(purpose);
    }

    if (createBoard) {
        cy.get('#add-board-to-channel').should('be.visible');
        cy.findByTestId('add-board-to-channel-check').then((el) => {
            if (el && !el.hasClass('checked')) {
                el.click();
                cy.get('.templates-selector').find('input').click({force: true});
                cy.findByText(createBoard).scrollIntoView().should('be.visible').click({force: true});
            }
        });
    }
    cy.findByText('Create channel').click();
    cy.get('#new-channel-modal').should('not.exist');
    cy.get('#channelIntro').should('be.visible');
    return cy.wrap({name: channelName});
});

Cypress.Commands.add('uiAddUsersToCurrentChannel', (usernameList) => {
    if (usernameList.length) {
        cy.get('#channelHeaderTitle').click();
        cy.get('#channelMembers').click();
        cy.uiGetButton('Add').click();
        cy.get('#addUsersToChannelModal').should('be.visible');
        usernameList.forEach((username) => {
            cy.get('#selectItems input').typeWithForce(`@${username}{enter}`);
        });
        cy.get('#saveItems').click();
        cy.get('#addUsersToChannelModal').should('not.exist');
    }
});

Cypress.Commands.add('uiInviteUsersToCurrentChannel', (usernameList) => {
    if (usernameList.length) {
        cy.get('#channelHeaderTitle').click();
        cy.get('#channelMembers').click();
        cy.uiGetButton('Add').click();
        cy.get('#addUsersToChannelModal').should('be.visible');
        usernameList.forEach((username) => {
            cy.get('#selectItems input').typeWithForce(`@${username}{enter}`);
        });
        cy.get('#saveItems').click();
        cy.get('#addUsersToChannelModal').should('not.exist');
    }
});

Cypress.Commands.add('uiArchiveChannel', () => {
    cy.get('#channelHeaderTitle').click();
    cy.get('#channelArchiveChannel').click();
    return cy.get('#deleteChannelModalDeleteButton').click();
});

Cypress.Commands.add('uiUnarchiveChannel', () => {
    cy.get('#channelHeaderTitle').should('be.visible').click();
    cy.get('#channelUnarchiveChannel').should('be.visible').click();
    return cy.get('#unarchiveChannelModal').findByRole('button', {name: /^Unarchive/}).should('be.visible').click();
});

Cypress.Commands.add('uiLeaveChannel', (isPrivate = false) => {
    cy.get('#channelHeaderTitle').click();

    if (isPrivate) {
        cy.get('#channelLeaveChannel').click();
        return cy.get('#confirmModalButton').click();
    }

    return cy.get('#channelLeaveChannel').click();
});

Cypress.Commands.add('goToDm', (username) => {
    cy.uiAddDirectMessage().click({force: true});

    // # Start typing part of a username that matches previously created users
    cy.get('#selectItems input').typeWithForce(username);
    cy.findByRole('dialog', {name: 'Direct Messages'}).should('be.visible').wait(TIMEOUTS.ONE_SEC);
    cy.findByRole('combobox', {name: 'Search for people'}).
        typeWithForce(username).
        wait(TIMEOUTS.ONE_SEC).
        typeWithForce('{enter}');

    // # Save the selected item
    return cy.get('#saveItems').click().wait(TIMEOUTS.HALF_SEC);
});
