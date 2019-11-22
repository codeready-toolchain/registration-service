import * as React from 'react';
import {
    Button,
    Modal,
    PageSection,
    Text
} from '@patternfly/react-core';
import { CopyrightIcon } from '@patternfly/react-icons';
import './ProvisionTOS.scss'
import { Redirect } from 'react-router';

const ProvisionTOS: React.FC<{}> = () => {
    const [showModal, setShowModal] = React.useState(false);
    const [accepted, setAccepted] = React.useState(false);


    if (accepted) {
        if (window.keycloak.authenticated) {
            return <Redirect to="/Provision" />
        } else {
            window.sessionStorage.setItem('crtcAction', 'PROVISION');
            window.keycloak.login({redirectUri: location.origin + '/Provision'});
            return null;
        }
    } else {
        return (
            <PageSection noPadding={true}>
                <div className="provision-section">
                    <Button variant="primary" onClick={() => setShowModal(true)}>Get Started with CodeReady Toolchain</Button>
                </div>
                <div className="provision-tnc-section">
                    <Button variant="link" onClick={() => setShowModal(true)}>Terms and Conditions</Button>
                    <CopyrightIcon></CopyrightIcon>
                    <Text component="small"> 2020, Red Hat</Text>
                </div>
                <Modal
                    isLarge
                    title="CodeReady Toolchain Terms of Service Agreement"
                    isOpen={showModal}
                    onClose={() => setShowModal(false)}
                    actions={[
                        <Button key="confirm" variant="primary" onClick={() => setAccepted(true)}>
                            Accept
                    </Button>,
                        <Button key="cancel" variant="link" onClick={() => setShowModal(false)}>
                            Cancel
                    </Button>
                    ]}
                    isFooterLeftAligned
                >
                    Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore
                    magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo
                    consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla
                    pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id
                    est laborum.
            </Modal>
            </PageSection>
        );
    };
}

export default ProvisionTOS;
