import * as React from 'react';
import {Stack, StackItem} from '@patternfly/react-core';
import Provisioner from './Provisioner';
import Marketing from './Marketing';
import MarketingData from './MarketingData';

const Home: React.FC<{}> = () => {
    return (
        <Stack>
            <StackItem>
                <Provisioner />
            </StackItem>
            <StackItem>
                <Marketing materials={MarketingData.materials} />
            </StackItem>
        </Stack>
    );
};

export default Home;