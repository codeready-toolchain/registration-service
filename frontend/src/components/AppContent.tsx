import * as React from 'react';
import { Route, Switch } from 'react-router-dom';
import Home from './Home';
import Provisioner from './Provisioner';
import Dashboard from './Dashboard';

const AppContent: React.FC<{}> = () => {
    return (
        <Switch>
            <Route exact path="/Provision" component={Provisioner} />
            <Route exact path="/Dashboard" component={Dashboard} />
            <Route exact path="/Home" component={Home} />
            <Route exact path="/" />
        </Switch>
    );
};

export default AppContent;
