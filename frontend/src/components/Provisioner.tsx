import * as React from 'react';
import { Stack, StackItem } from '@patternfly/react-core';
import MaterialList from './MaterialList';
import MarketingData from './MarketingData';
import axios from 'axios';
import { Redirect } from 'react-router-dom';
import { Spinner } from '@patternfly/react-core/dist/esm/experimental';
import { OkIcon } from '@patternfly/react-icons'

enum ProvisionStatus {
    UNPROVISIONED,
    PROCESSING,
    PENDING,
    SUCCESS,
    FAILED
}

export interface ProvisionerMessage {
    [name: string]: string;
};

/* const provisionerMsg = {
    "PROCESSING": "Your CodeReady Toolchain account is being provisioned",
    "PENDING": "Your CodeReady Toolchain account approval is pending",
    "SUCCESS": "Your CodeReady Toolchain account has been provisioned",
    "FAILED": "Internal server error. Please try again later."
} as ProvisionerMessage; */

const Provisioner: React.FC<{}> = () => {

    const [status, setStatus] = React.useState(ProvisionStatus.UNPROVISIONED);

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
                    .catch((error) => {
                        console.log('Polling failed', error);
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
            .catch((error) => {
                if (error.response && error.response.status && error.response.status === 404) {
                    setUserSignup()
                        .then((response) => {
                            setStatus(ProvisionStatus.PROCESSING);
                            intervalId.current = startStatusPolling();
                        })
                        .catch((error) => {
                            console.log('POST /usersignup failed', error.message);
                            setStatus(ProvisionStatus.FAILED);
                        });
                } else {
                    console.log('GET /usersignup failed - ', error.message);
                    setStatus(ProvisionStatus.FAILED);
                }
            });

        return () => {
            stopStatusPolling();
        };
    }, []);

    if (!window.keycloak.authenticated) {
        return <Redirect to="/Home" />;
    }

    /* const getProvisionStatusMsg = () => {
        return provisionerMsg[status];
    }; */

    /* const icon =
        status === ProvisionStatus.PROCESSING
            ? 'Your CodeReady Toolchain account is being provisioned'
            : status === ProvisionStatus.PENDING
                ? 'Your CodeReady Toolchain account is pending approval'
                : 'Your CodeReady Toolchain account has been provisioned'; */

    return (
        <Stack>
            <StackItem>
                <div className="provision-section">
                    {status === ProvisionStatus.PROCESSING && (
                        <span >
                            <Spinner size="lg" />  Your CodeReady Toolchain account is being provisioned
                        </span>
                    )}
                    {status === ProvisionStatus.PENDING && (
                        <span >
                            <Spinner size="lg" />  Your CodeReady Toolchain account is pending approval
                        </span>
                    )}
                    {status === ProvisionStatus.FAILED && (
                        <span >
                            Internal server error. Please try again later
                        </span>
                    )}
                    {status === ProvisionStatus.SUCCESS && (
                        <div>
                            <span>
                                <OkIcon /> Your CodeReady Toolchain account has been provisioned
                        </span>
                            <Redirect to="/Dashboard" />
                        </div>
                    )}
                </div>
            </StackItem>
            <StackItem>
                <MaterialList materials={MarketingData.materials} />
            </StackItem>
        </Stack>);
};

export default Provisioner;
