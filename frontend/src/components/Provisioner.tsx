import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import MaterialList from './MaterialList';
import MarketingData from './MarketingData';
import axios from 'axios';
import { Redirect } from 'react-router-dom';

enum ProvisionStatus {
    PROCESSING,
    PENDING,
    SUCCESS,
    FAILED,
}

export interface ProvisionerMessage {
    [name: string]: string;
};

const provisionerMsg = {
    "PROCESSING": "Your CodeReady Toolchain account is being provisioned",
    "PENDING": "Your CodeReady Toolchain account approval is pending",
    "SUCCESS": "Your CodeReady Toolchain account has been provisioned",
    "FAILED": "Your CodeReady Toolchain account provisioning has failed. Please contact administrator"
} as ProvisionerMessage;

const Provisioner: React.FC<{}> = () => {

    const [status, setStatus] = React.useState(ProvisionStatus.PROCESSING);

    const intervalId = React.useRef(null);

    React.useEffect(() => {
        const getUserSignup = () => {
            return axios.get('/api/v1/signup');
        };

        const updateStatus = (status) => {
            console.log('Status :', status);
            if (status['ready']) {
                setStatus(ProvisionStatus.SUCCESS);
                stopStatusPolling();
            } else {
                if (status['reason'] === 'PendingApproval') {
                    setStatus(ProvisionStatus.PENDING);
                } else {
                    setStatus(ProvisionStatus.PROCESSING);
                }
                intervalId.current = startStatusPolling();
            }
        };

        const startStatusPolling = () => {
            return setInterval(() => {
                getUserSignup()
                    .then(({ data }) => {
                        updateStatus(data.status);
                    })
                    .catch((status) => {
                        console.log('Polling failed', status);
                        setStatus(ProvisionStatus.FAILED);
                    });
            }, 60000);
        };

        const stopStatusPolling = () => {
            if (intervalId.current) {
                clearInterval(intervalId.current);
                intervalId.current = null;
            }
        };

        const setUserSignup = () => {
            return axios.post('/api/v1/signup', null);
        };

        window.keycloak.authenticated && getUserSignup()
            .then(({ data }) => {
                updateStatus(data.status);
            })
            .catch(() => {
                /* TODO: Need to check state in redirect URL before provisioning */
                setUserSignup();
                intervalId.current = startStatusPolling();
            });

        return () => {
            stopStatusPolling();
        };
    }, []);

    if (!window.keycloak.authenticated) {
        return <Redirect to="/Home" />;
    }

    return status !== ProvisionStatus.SUCCESS
        ? (<Stack>
            <StackItem>
                <div className="provision-section">{provisionerMsg[status]}</div>
            </StackItem>
            <StackItem>
                <MaterialList materials={MarketingData.materials} />
            </StackItem>
        </Stack>)
        : (<Redirect to="/Dashboard" />);
};

export default Provisioner;
