import React, { useState } from 'react'

import Modal, { ModalTitle, ModalContent } from '../Modal'
import Stepper from '../Stepper'
import SlidingPane from '../SlidingPane'
import NameAndIconPane from '../AddServer/NameAndIconPane'
import ChooseHostingPane from '../AddServer/ChooseHostingPane'
import ConfigureHostingPane from '../AddServer/ConfigureHostingPane'
import ConfirmationPane from '../AddServer/ConfirmationPane'

function AddServerModal({ onDismiss }) {
    const [activeStep, setActiveStep] = useState(0)
    const [requestValues, setRequestValues] = useState({})
    const [provider, setProvider] = useState()

    const steps =
        !!provider && provider !== 'none'
            ? ['Name and icon', 'Hosting', 'Configure hosting', 'Confirmation']
            : ['Name and icon', 'Hosting', 'Confirmation']

    const closeModal = () => {
        onDismiss()
        setActiveStep(0)
    }

    const onClickNext = () => {
        if (activeStep === steps.length - 1) {
            return
        }
        setActiveStep(activeStep + 1)
    }

    const onClickBack = () => {
        if (activeStep === 0) {
            return
        }
        setActiveStep(activeStep - 1)
    }

    const panes = [
        {
            title: 'Name and icon',
            width: '500px',
            height: '190px',
            content: (
                <NameAndIconPane
                    key="one"
                    setRequestValues={setRequestValues}
                    onClickBack={onClickBack}
                    onClickNext={onClickNext}
                />
            ),
        },
        {
            title: 'Hosting',
            width: '640px',
            height: '390px',
            content: (
                <ChooseHostingPane
                    key="two"
                    provider={provider}
                    setProvider={setProvider}
                    onClickBack={onClickBack}
                    onClickNext={onClickNext}
                />
            ),
        },
        {
            title: 'Configure hosting',
            width: '600px',
            height: '690px',
            content: (
                <ConfigureHostingPane
                    key="three"
                    setRequestValues={setRequestValues}
                    onClickBack={onClickBack}
                    onClickNext={onClickNext}
                />
            ),
        },
        {
            title: 'Confirmation',
            width: 600,
            height: 510,
            content: (
                <ConfirmationPane
                    key="four"
                    provider={provider}
                    requestValues={requestValues}
                    onClickBack={onClickBack}
                    closeModal={closeModal}
                />
            ),
        },
    ].filter((p) => steps.includes(p.title))

    return (
        <Modal modalKey="add server" closeModal={closeModal}>
            <ModalTitle closeModal={closeModal}>Create a Server</ModalTitle>
            <ModalContent>
                <Stepper activeStep={activeStep} steps={steps} />
                <SlidingPane activePane={activeStep} panes={panes} />
            </ModalContent>
        </Modal>
    )
}

export default AddServerModal
