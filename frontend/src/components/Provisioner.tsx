import * as React from 'react';
import {
    Button,
    Modal,
    PageSection,
    Text
} from '@patternfly/react-core';
import { CopyrightIcon } from '@patternfly/react-icons';
import axios from 'axios';
import './Provisioner.scss'

interface ProvisionProps {
    data?: {};
}

interface ProvisionStates {
    showToSModal: boolean;
}
class Provision extends React.Component<ProvisionProps, ProvisionStates> {
    constructor(props: Readonly<ProvisionProps>) {
        super(props);
        this.state = {
            showToSModal: false
        };

        this.showToSModal = this.showToSModal.bind(this);
        this.hideToSModal = this.hideToSModal.bind(this);
        this.handleToSConfirmation = this.handleToSConfirmation.bind(this);
        this.getUserSignup = this.getUserSignup.bind(this);
        this.setUserSignup = this.setUserSignup.bind(this);
        this.handleLogout = this.handleLogout.bind(this);
    }

    showToSModal() {
        this.setState({ showToSModal: true });
    }

    hideToSModal() {
        this.setState({ showToSModal: false });
    }

    handleToSConfirmation = () => {
        this.hideToSModal();
        window.keycloak.login()
            .success((res: any) => {
                console.log(res);
            })
            .error((err: any) => {
                console.error(err);
            });
    }

    getUserSignup = () => {
        axios.defaults.headers.common['Authorization'] = "Bearer " + window.keycloak.token;
        axios.get("/api/v1/signup")
            .then(res => {
                console.log(res);
            })
            .catch(err => {
                console.error(err);
            });
    }
    setUserSignup = () => {
        axios.defaults.headers.common['Authorization'] = "Bearer " + window.keycloak.token;
        axios.post("/api/v1/signup", null)
            .then(res => {
                console.log(res);
            })
            .catch(err => {
                console.error(err);
            });
    }

    handleLogout = () => {
        window.keycloak.logout()
            .success((res: any) => {
                console.log(res);
            })
            .error((err: any) => {
                console.error(err);
            });
    }

    render() {
        return (
            <PageSection noPadding={true}>
                <div className="provision-section">
                    <Button variant="primary" onClick={this.showToSModal}>Get Started with CodeReady Toolchain</Button>
                    <Button variant="secondary" onClick={this.getUserSignup}>GET User Signup</Button>
                    <Button variant="tertiary" onClick={this.setUserSignup}>POST User Signup</Button>
                    <Button variant="tertiary" onClick={this.handleLogout}>Logout</Button>
                </div>
                <div className="provision-tnc-section">
                    <Button variant="link" onClick={this.showToSModal}>Terms and Conditions</Button>
                    <CopyrightIcon></CopyrightIcon>
                    <Text component="small"> 2020, Red Hat</Text>
                </div>
                <Modal
                    isLarge
                    title="CodeReady Toolchain Terms of Service Agreement"
                    isOpen={this.state.showToSModal}
                    onClose={this.hideToSModal}
                    actions={[
                        <Button key="confirm" variant="primary" onClick={this.handleToSConfirmation}>
                            Accept
                    </Button>,
                        <Button key="cancel" variant="link" onClick={this.hideToSModal}>
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

export default Provision;
